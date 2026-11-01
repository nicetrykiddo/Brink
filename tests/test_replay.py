import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
import zipfile

from brink.replay import command_category, detect, load_events


class ClassificationTests(unittest.TestCase):
    def test_mechanisms(self):
        self.assertEqual(command_category("Bash", "cd /tmp/reactor/pkg && npm pack next@15.0.3"), "local_research")
        self.assertEqual(command_category("Bash", "curl -m 3 http://10.10.10.10:3000/"), "live_probe")
        self.assertEqual(command_category("Bash", "./bin/contextforge checkpoint"), "memory")
        self.assertEqual(command_category("Read", "/tmp/x"), "other_tool")

    def test_research_drift(self):
        meta = {"started_ms": 1000, "ended_ms": 320000}
        base = {"kind": "tool", "end": 2000, "tool": "Bash", "duration_ms": 0,
                "failed": False, "masked_error": False, "checkpoint_success": False}
        events = [dict(base, ts=1000, category="live_probe")]
        events += [dict(base, ts=10000 + i * 35000, end=10001 + i * 35000,
                        category="local_research") for i in range(9)]
        report = detect(events, meta)
        drift = [a for a in report["alerts"] if a["code"] == "local_research_drift"]
        self.assertEqual(len(drift), 1)
        self.assertGreater(drift[0]["elapsed_s"], 180)

    def test_live_probe_resets_drift(self):
        meta = {"started_ms": 0, "ended_ms": 500000}
        base = {"kind": "tool", "tool": "Bash", "duration_ms": 0,
                "failed": False, "masked_error": False, "checkpoint_success": False}
        events = [dict(base, ts=0, end=0, category="live_probe")]
        events += [dict(base, ts=200000 + i*1000, end=200000+i*1000,
                        category="local_research") for i in range(7)]
        events += [dict(base, ts=210000, end=210000, category="live_probe")]
        events += [dict(base, ts=401000 + i*1000, end=401000+i*1000,
                        category="local_research") for i in range(7)]
        self.assertFalse(any(a["code"] == "local_research_drift" for a in detect(events,meta)["alerts"]))

    def test_model_latency_and_privacy(self):
        events = [{"kind": "model", "ts": 0, "end": 131000, "duration_ms": 131000}]
        report = detect(events, {"started_ms": 0, "ended_ms": 131000})
        self.assertEqual(report["alerts"][0]["code"], "slow_model_call")
        self.assertEqual(report["alerts"][0]["elapsed_s"], 131.0)
        self.assertNotIn("request", json.dumps(report))

    def test_real_zip_shape_without_secrets_leaking(self):
        with TemporaryDirectory() as tmp:
            archive = Path(tmp)/"trace.zip"
            usage = {"tool_call_id":"call_a", "started_at":1000, "completed_at":1500,
                     "duration_ms":500, "tool_name":"Bash", "exit_code":0}
            part = {"time_created":1000,"data":json.dumps({"type":"tool","callID":"call_a","tool":"Bash",
                    "state":{"input":{"command":"curl http://SECRET_API_KEY@host"},
                             "output":"SECRET_API_KEY", "status":"completed"}})}
            model = {"started_at": 1600,"completed_at":70000,"duration_ms":68400,"status":"completed"}
            with zipfile.ZipFile(archive,"w") as z:
                z.writestr("database/part.jsonl",json.dumps(part)+"\n")
                z.writestr("database/tool_usage.jsonl",json.dumps(usage)+"\n")
                z.writestr("database/model_usage.jsonl",json.dumps(model)+"\n")
            events,meta = load_events(archive)
            report = detect(events,meta)
            self.assertEqual(report["tool_calls"], 1)
            self.assertEqual(report["model_calls"], 1)
            self.assertNotIn("SECRET_API_KEY",json.dumps(report))
            self.assertEqual(report["category_counts"],{"live_probe":1})

    def test_cancelled_model_time_included(self):
        events = [{"kind":"model", "ts":0,"end":120000, "duration_ms":120000, "status":"cancelled"}]
        report = detect(events,{"started_ms":0,"ended_ms":120000})
        self.assertEqual(report["model_calls"],1)
        self.assertEqual(report["cancelled_model_calls"],1)
        self.assertEqual(report["model_wall_s_sum"],120.0)

    def test_masked_errors_detected(self):
        base = {"kind":"tool", "tool":"Bash", "ts":0, "end":10, "category":"memory",
                "failed":False,"masked_error":True,"checkpoint_success":False}
        report = detect([base], {"started_ms":0,"ended_ms":10})
        self.assertEqual(report["alerts"][0]["code"],"masked_tool_error")


if __name__ == "__main__":
    unittest.main()
