package desktop

import "testing"

func TestUXMinimumWindowDimensions(t *testing.T) {
  tests := []struct{ name string; cfg Config; w,h int32 }{
    {"main default",Config{},960,600},
    {"main explicit",Config{MinWidth:960,MinHeight:600},960,600},
    {"detached terminal",Config{MinWidth:640,MinHeight:400},640,400},
    {"detached editor",Config{MinWidth:640,MinHeight:400,RequireCloseApproval:true},640,400},
  }
  for _,tc:=range tests { t.Run(tc.name,func(t *testing.T) {
    if w,h:=cfgMinimumWidth(tc.cfg),cfgMinimumHeight(tc.cfg);w!=tc.w||h!=tc.h {t.Fatalf("minimum %dx%d, want %dx%d",w,h,tc.w,tc.h)}
  })}
}
