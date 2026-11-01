// Package replay reconstructs ZCode tool/model telemetry without retaining any
// model prompt, command body, tool output, credential, or flag in its reports.
package replay

import (
 "archive/zip"
 "bufio"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "math"
 "os"
 "regexp"
 "sort"
 "strings"
)
var localHints=[]string{"/pkg/","npm pack","npm view","npm install","rsc_patch","harness","react-server-dom","server.node.","node -e","conditions.js","exp1.js","exp2.js","diff ","patch.diff","shim/node_modules","package.json","app-render/action-handler","server-action-request-meta"}
var liveHints=[]string{"curl ","nmap ","gobuster ","ffuf ","feroxbuster ","ssh ","sshpass ","nc ","ping ","dig ","nikto ","whatweb ","httpx ","wfuzz ","sqlmap "}
var memoryHints=[]string{"contextforge",".ctx/"}
var localOtherHints=[]string{"find ","grep ","sed ","cat ","ls ","pwd","wc "}
var diagnostic=regexp.MustCompile("(?i)command not found|no such file or directory|the following arguments are required|traceback \\(most recent call last\\)")
func hasAny(text string,hints []string)bool{for _,x:=range hints{if strings.Contains(text,x){return true}};return false}
func Category(tool,command string)string{
 if tool!="Bash"{return "other_tool"}
 c:=strings.ToLower(command)
 switch{
 case hasAny(c,memoryHints):return "memory"
 case hasAny(c,localHints):return "local_research"
 case hasAny(c,liveHints):return "live_probe"
 case hasAny(c,localOtherHints):return "local_other"
 default:return "other"
 }
}
func object(v any)map[string]any{m,ok:=v.(map[string]any);if !ok{return map[string]any{}};return m}
func str(v any)string{if v==nil{return ""};s,ok:=v.(string);if ok{return s};return fmt.Sprint(v)}
func number(v any)int64{switch x:=v.(type){case float64:return int64(x);case int:return int64(x);case int64:return x;case json.Number:i,_:=x.Int64();return i;case string:var i int64;fmt.Sscan(x,&i);return i;default:return 0}}
func valueOrDefault(m map[string]any,k string,def int64)int64{if m[k]==nil{return def};return number(m[k])}
func rnd(v float64)float64{return math.Round(v*10)/10}

type Event struct{Kind string;TS,End,Duration int64;Tool,Category,Status string;Failed,MaskedError,CheckpointSuccess bool}
type Metadata struct{Started,Ended int64;SessionCount,ToolRecords,ModelRecords int}
type Alert struct{
 ElapsedS float64 `json:"elapsed_s"`
 Code string `json:"code"`
 Message string `json:"message"`
 Evidence map[string]any `json:"evidence"`
 Intervention string `json:"intervention"`
}
type Report struct{
 SchemaVersion int `json:"schema_version"`
 Privacy string `json:"privacy"`
 Scope string `json:"scope"`
 DurationS float64 `json:"duration_s"`
 ModelCalls int `json:"model_calls"`
 CancelledModelCalls int `json:"cancelled_model_calls"`
 ToolCalls int `json:"tool_calls"`
 ModelWallSSum float64 `json:"model_wall_s_sum"`
 CategoryCounts map[string]int `json:"category_counts"`
 MaskedToolErrors int `json:"masked_tool_errors"`
 VerifiedCheckpointResponses int `json:"verified_checkpoint_responses"`
 Alerts []Alert `json:"alerts"`
}
type Options struct{ModelBudgetS,ResearchBudgetS int64;MinResearchCalls,FailWindow,FailLimit int}
func Defaults()Options{return Options{ModelBudgetS:60,ResearchBudgetS:180,MinResearchCalls:8,FailWindow:8,FailLimit:3}}

func readJSONLines(f *zip.File,callback func(map[string]any)error)error{
 rc,err:=f.Open();if err!=nil{return err};defer rc.Close()
 scanner:=bufio.NewScanner(rc);scanner.Buffer(make([]byte,64*1024),20*1024*1024)
 for scanner.Scan(){
  if len(strings.TrimSpace(scanner.Text()))==0{continue}
  var m map[string]any
  if err:=json.Unmarshal(scanner.Bytes(),&m);err!=nil{return fmt.Errorf("invalid JSONL entry in %s: %w",f.Name,err)}
  if err:=callback(m);err!=nil{return err}
 }
 return scanner.Err()
}
func Load(path string)([]Event,Metadata,error){
 src,err:=os.Open(path);if err!=nil{return nil,Metadata{},err};defer src.Close()
 stat,err:=src.Stat();if err!=nil{return nil,Metadata{},err}
 archive,err:=zip.NewReader(src,stat.Size());if err!=nil{return nil,Metadata{},err}
 entries:=map[string]*zip.File{};for _,f:=range archive.File{entries[f.Name]=f}
 if entries["database/part.jsonl"]==nil||entries["database/tool_usage.jsonl"]==nil{return nil,Metadata{},errors.New("expected ZCode full-session ZIP with part and tool_usage JSONL")}
 tools:=map[string]map[string]any{};meta:=Metadata{};events:=[]Event{}
 if err:=readJSONLines(entries["database/tool_usage.jsonl"],func(m map[string]any)error{tools[str(m["tool_call_id"])]=m;meta.ToolRecords++;return nil});err!=nil{return nil,meta,err}
 if f:=entries["database/model_usage.jsonl"];f!=nil{
  err=readJSONLines(f,func(m map[string]any)error{
   ts:=number(m["started_at"]);duration:=number(m["duration_ms"]);end:=valueOrDefault(m,"completed_at",ts+duration)
   events=append(events,Event{Kind:"model",TS:ts,End:end,Duration:duration,Status:str(m["status"])})
   meta.ModelRecords++;return nil
  });if err!=nil{return nil,meta,err}
 }
 if f:=entries["database/session.jsonl"];f!=nil{
  first:=true
  err=readJSONLines(f,func(m map[string]any)error{if first{meta.Ended=number(m["time_updated"]);first=false};meta.SessionCount++;return nil})
  if err!=nil{return nil,meta,err}
 }
 err=readJSONLines(entries["database/part.jsonl"],func(item map[string]any)error{
  data:=item["data"]
  if s,ok:=data.(string);ok{var v any;if json.Unmarshal([]byte(s),&v)!=nil{return nil};data=v}
  d:=object(data);if str(d["type"])!="tool"{return nil}
  state:=object(d["state"]);command:=str(object(state["input"])["command"])
  tool:=str(d["tool"]);if tool==""{tool="unknown"}
  stat:=tools[str(d["callID"])]
  ts:=valueOrDefault(stat,"started_at",number(item["time_created"]))
  duration:=number(stat["duration_ms"]);end:=valueOrDefault(stat,"completed_at",ts+duration)
  output,outputIsString:=state["output"].(string)
  exitCode,exists:=stat["exit_code"]
  zeroExit:=!exists||exitCode==nil||number(exitCode)==0
  category:=Category(tool,command)
  events=append(events,Event{Kind:"tool",TS:ts,End:end,Duration:duration,Tool:tool,Category:category,
   Failed:!zeroExit||str(stat["error_type"])!="",
   MaskedError:outputIsString&&diagnostic.MatchString(output)&&zeroExit,
   CheckpointSuccess:tool=="Bash"&&strings.Contains(strings.ToLower(command),"contextforge")&&outputIsString&&(strings.Contains(output,"Checkpoint saved")||strings.Contains(output," [verified]"))})
  return nil
 });if err!=nil{return nil,meta,err}
 sort.SliceStable(events,func(i,j int)bool{if events[i].TS==events[j].TS{return events[i].Kind<events[j].Kind};return events[i].TS<events[j].TS})
 started:=int64(0);ended:=int64(0)
 for _,e:=range events{if e.TS>0&&(started==0||e.TS<started){started=e.TS};if e.End>ended{ended=e.End}}
 meta.Started=started;if ended>meta.Ended{meta.Ended=ended};if meta.Ended==0{meta.Ended=started}
 return events,meta,nil
}
func Detect(events []Event,meta Metadata,opts Options)Report{
 if opts.ModelBudgetS<=0{opts.ModelBudgetS=60}
 if opts.ResearchBudgetS<=0{opts.ResearchBudgetS=180}
 if opts.MinResearchCalls<=0{opts.MinResearchCalls=8}
 if opts.FailWindow<=0{opts.FailWindow=8}
 if opts.FailLimit<=0{opts.FailLimit=3}
 report:=Report{SchemaVersion:1,Privacy:"Metadata only; no prompts, commands, tool outputs, or raw model traces.",Scope:"Retrospective replay. No real-time alerts or interventions were performed.",CategoryCounts:map[string]int{},Alerts:[]Alert{}}
 var lastLive int64;localSince:=0;driftReported:=false
 failures:=[]bool{};var modelTotal int64
 add:=func(ts int64,code,message string,evidence map[string]any){
  report.Alerts=append(report.Alerts,Alert{ElapsedS:rnd(math.Max(0,float64(ts-meta.Started)/1000)),Code:code,Message:message,Evidence:evidence,Intervention:"advisory_only"})
 }
 for _,e:=range events{
  if e.Kind=="model"{
   report.ModelCalls++;modelTotal+=e.Duration
   if e.Status=="cancelled"{report.CancelledModelCalls++}
   if e.Duration>=opts.ModelBudgetS*1000{add(e.End,"slow_model_call","Model call exceeded its latency budget.",map[string]any{"duration_s":rnd(float64(e.Duration)/1000),"budget_s":opts.ModelBudgetS})}
   continue
  }
  category:=e.Category;report.CategoryCounts[category]++;report.ToolCalls++
  if category=="live_probe"{lastLive=e.End;localSince=0;driftReported=false
  }else if category=="local_research"{
   localSince++;base:=lastLive;if base==0{base=meta.Started}
   elapsed:=e.TS-base;if elapsed<0{elapsed=0}
   if !driftReported&&localSince>=opts.MinResearchCalls&&elapsed>=opts.ResearchBudgetS*1000{
    add(e.TS,"local_research_drift","Extended local investigation without a new live target test.",map[string]any{"research_calls_since_live":localSince,"seconds_since_live":rnd(float64(elapsed)/1000)})
    driftReported=true
   }
  }
  if e.MaskedError{report.MaskedToolErrors++;add(e.End,"masked_tool_error","Tool output contains an error despite a successful or missing exit code.",map[string]any{"tool":e.Tool,"category":category})}
  if e.CheckpointSuccess{report.VerifiedCheckpointResponses++}
  failures=append(failures,e.Failed);if len(failures)>opts.FailWindow{failures=failures[1:]}
  if len(failures)==opts.FailWindow{
   n:=0;for _,f:=range failures{if f{n++}}
   if n>=opts.FailLimit{
    if len(report.Alerts)==0||report.Alerts[len(report.Alerts)-1].Code!="tool_failure_burst"{add(e.End,"tool_failure_burst","Several recent tool calls failed.",map[string]any{"failed_in_recent_window":n,"window":opts.FailWindow})}
    failures=nil
   }
  }
 }
 report.ModelWallSSum=rnd(float64(modelTotal)/1000)
 report.DurationS=rnd(math.Max(0,float64(meta.Ended-meta.Started)/1000))
 sort.SliceStable(report.Alerts,func(i,j int)bool{return report.Alerts[i].ElapsedS<report.Alerts[j].ElapsedS})
 return report
}
func Summary(r Report)string{
 var b strings.Builder
 fmt.Fprintf(&b,"Brink offline replay | duration %.1fm | %d model calls | %d tools\n",r.DurationS/60,r.ModelCalls,r.ToolCalls)
 fmt.Fprintf(&b,"Model-call wall time (sum): %.1fm (%d cancelled requests included)\n",r.ModelWallSSum/60,r.CancelledModelCalls)
 keys:=make([]string,0,len(r.CategoryCounts));for k:=range r.CategoryCounts{keys=append(keys,k)};sort.Strings(keys)
 cats:=make([]string,0,len(keys));for _,k:=range keys{cats=append(cats,fmt.Sprintf("%s=%d",k,r.CategoryCounts[k]))}
 fmt.Fprintf(&b,"Mechanisms: %s\nAlerts: %d (retrospective, advisory only)",strings.Join(cats,", "),len(r.Alerts))
 for _,a:=range r.Alerts{fmt.Fprintf(&b,"\n  +%.1fm  %s  %v",a.ElapsedS/60,a.Code,a.Evidence)}
 return b.String()
}
func WriteJSON(w io.Writer,r Report)error{enc:=json.NewEncoder(w);enc.SetIndent("","  ");return enc.Encode(r)}
