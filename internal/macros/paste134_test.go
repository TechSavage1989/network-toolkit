package macros

import (
  "strings"
  "testing"
)

func TestPaste134CaptureAssociatesSentCommandsAndOutput(t *testing.T) {
 e,b:=paste127Fixture(t)
 run,err:=e.StartPaste(PasteRequest{Target:refs("synthetic")[0],Text:"show one\n\nshow two\n"})
 if err!=nil {t.Fatal(err)}
 awaitWrite(t,b,"synthetic","show one\r")
 b.emit("synthetic","show one\r\nONE\r\nfixture# ")
 awaitWrite(t,b,"synthetic","\r")
 b.emit("synthetic","\r\nfixture# ")
 awaitWrite(t,b,"synthetic","show two\r")
 b.emit("synthetic","show two\r\nTWO\r\nfixture# ")
 result:=awaitRun(t,e,run.ID,finished)
 if result.State!="completed" {t.Fatalf("run: %+v",result)}
 capture,err:=e.PasteCapture(run.ID,"synthetic")
 if err!=nil {t.Fatal(err)}
 if capture.RunID!=run.ID||capture.SessionID!="synthetic"||capture.State!="completed"||len(capture.Steps)!=3 {t.Fatalf("capture: %+v",capture)}
 for i,command:=range []string{"show one","","show two"} {
  if capture.Steps[i].Index!=i+1||capture.Steps[i].Command!=command {t.Fatalf("step %d: %+v",i,capture.Steps[i])}
 }
 if !strings.Contains(capture.Steps[0].Output,"ONE")||strings.Contains(capture.Steps[0].Output,"TWO")||!strings.Contains(capture.Steps[2].Output,"TWO")||strings.Contains(capture.Steps[2].Output,"ONE") {t.Fatalf("crossed boundaries: %+v",capture.Steps)}
 if _,err:=e.PasteCapture(run.ID,"different");err==nil {t.Fatal("cross-session capture allowed")}
 if _,err:=e.PasteCapture(run.ID,"");err==nil {t.Fatal("empty session capture allowed")}
}

func TestPaste134StoppedRunDoesNotCaptureUnsentCommands(t *testing.T) {
 e,b:=paste127Fixture(t)
 run,err:=e.StartPaste(PasteRequest{Target:refs("synthetic")[0],Text:"first\nsecond\n"})
 if err!=nil {t.Fatal(err)}
 awaitWrite(t,b,"synthetic","first\r")
 b.emit("synthetic","first\r\nFIRST")
 if _,err=e.Control(run.ID,"stop");err!=nil {t.Fatal(err)}
 result:=awaitRun(t,e,run.ID,finished)
 if result.State!="stopped" {t.Fatalf("run: %+v",result)}
 capture,err:=e.PasteCapture(run.ID,"synthetic")
 if err!=nil {t.Fatal(err)}
 if len(capture.Steps)!=1||capture.Steps[0].Command!="first" {t.Fatalf("unsent lines captured: %+v",capture)}
 noWrite(t,b)
}
