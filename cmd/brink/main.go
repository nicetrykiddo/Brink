package main

import (
 "fmt"
 "os"
 "strconv"
 "github.com/nicetrykiddo/Brink/internal/replay"
)
func usage(){fmt.Fprintln(os.Stderr,"usage: brink replay SESSION.zip [--json OUTPUT.json] [--model-budget SECONDS] [--research-budget SECONDS]")}
func run(args []string) error {
 if len(args)==0||args[0]!="replay"{usage();return fmt.Errorf("expected replay subcommand")}
 var path,output string
 opts:=replay.Defaults()
 for i:=1;i<len(args);i++{
  a:=args[i]
  if a=="--json"||a=="--model-budget"||a=="--research-budget"{
   i++;if i>=len(args){return fmt.Errorf("missing value for %s",a)}
   val:=args[i]
   switch a{
    case "--json":output=val
    case "--model-budget":n,err:=strconv.ParseInt(val,10,64);if err!=nil||n<=0{return fmt.Errorf("invalid model budget %q",val)};opts.ModelBudgetS=n
    case "--research-budget":n,err:=strconv.ParseInt(val,10,64);if err!=nil||n<=0{return fmt.Errorf("invalid research budget %q",val)};opts.ResearchBudgetS=n
   }
  }else if len(a)>0&&a[0]=='-'{return fmt.Errorf("unknown flag %q",a)
  }else if path==""{path=a}else{return fmt.Errorf("unexpected argument %q",a)}
 }
 if path==""{usage();return fmt.Errorf("missing ZIP path")}
 events,meta,err:=replay.Load(path);if err!=nil{return err}
 report:=replay.Detect(events,meta,opts)
 fmt.Println(replay.Summary(report))
 if output!=""{
  f,err:=os.Create(output);if err!=nil{return err}
  err=replay.WriteJSON(f,report);cerr:=f.Close()
  if err!=nil{return err};if cerr!=nil{return cerr}
  fmt.Printf("Metadata-only JSON written to %s\n",output)
 }
 return nil
}
func main(){if err:=run(os.Args[1:]);err!=nil{fmt.Fprintln(os.Stderr,"brink:",err);os.Exit(2)}}
