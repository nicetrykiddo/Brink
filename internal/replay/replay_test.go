package replay

import (
 "archive/zip"
 "bytes"
 "encoding/json"
 "os"
 "path/filepath"
 "reflect"
 "strings"
 "testing"
)
func TestCategories(t *testing.T){
 cases:=map[string]string{"cd /tmp/reactor/pkg && npm pack next@15.0.3":"local_research","curl -m 3 http://target.invalid":"live_probe","./bin/contextforge checkpoint":"memory"}
 for command,expected:=range cases{if got:=Category("Bash",command);got!=expected{t.Fatalf("%s: %s != %s",command,got,expected)}}
 if Category("Read","x")!="other_tool"{t.Fatal("non Bash")}
}
func baseEvent(ts,end int64,cat string)Event{return Event{Kind:"tool",TS:ts,End:end,Tool:"Bash",Category:cat}}
func alertsWithCode(r Report,code string)[]Alert{out:=[]Alert{};for _,a:=range r.Alerts{if a.Code==code{out=append(out,a)}};return out}
func TestResearchDrift(t *testing.T){
 events:=[]Event{baseEvent(1000,2000,"live_probe")}
 for i:=0;i<9;i++{ts:=int64(10000+i*35000);events=append(events,baseEvent(ts,ts+1,"local_research"))}
 r:=Detect(events,Metadata{Started:1000,Ended:320000},Defaults());a:=alertsWithCode(r,"local_research_drift")
 if len(a)!=1||a[0].ElapsedS<=180{t.Fatalf("drift: %#v",a)}
}
func TestProbeResetsDrift(t *testing.T){
 events:=[]Event{baseEvent(0,0,"live_probe")}
 for i:=0;i<7;i++{ts:=int64(200000+i*1000);events=append(events,baseEvent(ts,ts,"local_research"))}
 events=append(events,baseEvent(210000,210000,"live_probe"))
 for i:=0;i<7;i++{ts:=int64(401000+i*1000);events=append(events,baseEvent(ts,ts,"local_research"))}
 if a:=alertsWithCode(Detect(events,Metadata{Started:0,Ended:500000},Defaults()),"local_research_drift");len(a)!=0{t.Fatalf("false drift %#v",a)}
}
func TestSlowAndCancelled(t *testing.T){
 e:=Event{Kind:"model",TS:0,End:131000,Duration:131000,Status:"cancelled"}
 r:=Detect([]Event{e},Metadata{Started:0,Ended:131000},Defaults())
 if r.ModelCalls!=1||r.CancelledModelCalls!=1||r.ModelWallSSum!=131||r.Alerts[0].Code!="slow_model_call"{t.Fatalf("wrong model accounting: %#v",r)}
}
func TestMaskedError(t *testing.T){
 e:=baseEvent(0,10,"memory");e.MaskedError=true;r:=Detect([]Event{e},Metadata{Started:0,Ended:10},Defaults())
 if len(r.Alerts)!=1||r.Alerts[0].Code!="masked_tool_error"{t.Fatal(r.Alerts)}
}
func TestFailureBurst(t *testing.T){
 events:=[]Event{}
 for i:=0;i<8;i++{e:=baseEvent(int64(i*1000),int64(i*1000),"other");e.Failed=i<3;events=append(events,e)}
 r:=Detect(events,Metadata{Started:0,Ended:8000},Defaults())
 if len(alertsWithCode(r,"tool_failure_burst"))!=1{t.Fatalf("no burst: %#v",r.Alerts)}
}
func TestZipAndPrivacy(t *testing.T){
 path:=filepath.Join(t.TempDir(),"trace.zip");f,err:=os.Create(path);if err!=nil{t.Fatal(err)};z:=zip.NewWriter(f)
 records:=map[string][]any{
  "database/tool_usage.jsonl":{map[string]any{"tool_call_id":"call_a","started_at":1000,"completed_at":1500,"duration_ms":500,"tool_name":"Bash","exit_code":0}},
  "database/part.jsonl":{map[string]any{"time_created":1000,"data":"{\"type\":\"tool\",\"callID\":\"call_a\",\"tool\":\"Bash\",\"state\":{\"input\":{\"command\":\"curl http://SECRET_API_KEY@host\"},\"output\":\"SECRET_API_KEY\",\"status\":\"completed\"}}"}},
  "database/model_usage.jsonl":{map[string]any{"started_at":1600,"completed_at":70000,"duration_ms":68400,"status":"completed"}},
 }
 for name,entries:=range records{
  w,err:=z.Create(name);if err!=nil{t.Fatal(err)}
  for _,rec:=range entries{b,err:=json.Marshal(rec);if err!=nil{t.Fatal(err)};w.Write(append(b,'\n'))}
 }
 z.Close();f.Close()
 events,meta,err:=Load(path);if err!=nil{t.Fatal(err)}
 r:=Detect(events,meta,Defaults());var out bytes.Buffer
 if err:=WriteJSON(&out,r);err!=nil{t.Fatal(err)}
 if r.ToolCalls!=1||r.ModelCalls!=1||!reflect.DeepEqual(r.CategoryCounts,map[string]int{"live_probe":1}){t.Fatalf("wrong result: %#v",r)}
 if strings.Contains(out.String(),"SECRET_API_KEY")||strings.Contains(out.String(),"curl"){t.Fatalf("leak in report: %s",out.String())}
}
