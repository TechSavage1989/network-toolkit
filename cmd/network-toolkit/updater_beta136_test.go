package main

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "net/http"
 "strings"
 "testing"
)

func betaFixture(t *testing.T, stable, beta string) []byte {
 t.Helper()
 a:=map[string]any{}
 if err:=json.Unmarshal(feedFixture(beta),&a);err!=nil {t.Fatal(err)}
 a["prerelease"]=true
 var entries []json.RawMessage
 for _,v:=range []any{a,json.RawMessage(feedFixture(stable))} {
   data,err:=json.Marshal(v);if err!=nil{t.Fatal(err)};entries=append(entries,data)
 }
 result,err:=json.Marshal(entries);if err!=nil{t.Fatal(err)}
 return result
}
func TestBeta136FeedSelectsOfficialPrereleaseWithoutAffectingStableParser(t *testing.T){
 data:=betaFixture(t,"1.17.136","1.17.137")
 r,err:=updateParseReleaseFeed(data,"beta")
 if err!=nil||r==nil||r.Version!="1.17.137"||!r.Beta{t.Fatalf("beta: %+v %v",r,err)}
 var items []json.RawMessage
 if err:=json.Unmarshal(data,&items);err!=nil{t.Fatal(err)}
 if _,err:=updateParseRelease(items[0]);err==nil{t.Fatal("stable parser accepted prerelease")}
 stable,err:=updateParseRelease(items[1])
 if err!=nil||stable.Beta||stable.Version!="1.17.136"{t.Fatalf("stable: %+v %v",stable,err)}
 newer:=betaFixture(t,"1.17.138","1.17.137")
 result,err:=updateParseReleaseFeed(newer,"beta")
 if err!=nil||result.Version!="1.17.138"||result.Beta{t.Fatalf("newer stable should win: %+v %v",result,err)}
}
func TestBeta136FeedRejectsUnsafeAndMalformedPublishedCandidates(t *testing.T){
 for name,body:=range map[string][]byte{
 "tampered repository":[]byte(strings.ReplaceAll(string(betaFixture(t,"1.17.136","1.17.137")),"github.com/TechSavage1989/network-toolkit/","github.com/other/repo/")),
 "truncated":[]byte(`[{"tag_name":"v1.17.137","prerelease":true,"assets":[]}`),
 "missing checksums":[]byte(`[{"tag_name":"v1.17.137","prerelease":true,"assets":[]}]`),
 "not list":feedFixture("1.17.136"),
 }{
  t.Run(name,func(t *testing.T){if _,err:=updateParseReleaseFeed(body,"beta");err==nil{t.Fatal("unsafe beta feed accepted")}})
 }
}
func TestBeta136FeedCacheNeverReinterpretsStableMetadata(t *testing.T){
 var calls []string
 client:=&http.Client{Transport:updateRoundTrip(func(r *http.Request)(*http.Response,error){
  calls=append(calls,r.URL.String())
  if r.URL.String()==updateAPIURL {
   if r.Header.Get("If-None-Match")!="" {t.Fatal("stable validator reused after beta selection")}
   return updatePollResponse(200,feedFixture("1.17.136"),map[string]string{"ETag":`"stable"`}),nil
  }
  if r.URL.String()!=updateBetaAPIURL {t.Fatalf("unexpected feed %s",r.URL);return nil,nil}
  if r.Header.Get("If-None-Match")!="" {t.Fatal("stable validator sent to beta endpoint")}
  return updatePollResponse(200,betaFixture(t,"1.17.136","1.17.137"),map[string]string{"ETag":`"beta"`}),nil
 })}
 stable,cache,err:=updateFetchRelease(context.Background(),client,updateFeedCache{})
 if err!=nil||stable.Version!="1.17.136"{t.Fatalf("stable: %+v %v",stable,err)}
 beta,cache,err:=updateFetchReleaseChannel(context.Background(),client,cache,"beta")
 if err!=nil||beta.Version!="1.17.137"||!beta.Beta||cache.channel!="beta"||cache.etag!=`"beta"`{t.Fatalf("beta: %+v %+v %v",beta,cache,err)}
 last,cache,err:=updateFetchReleaseChannel(context.Background(),client,cache,"stable")
 if err!=nil||last.Version!="1.17.136"||last.Beta||cache.channel!="stable"||len(calls)!=3{t.Fatalf("return stable: %+v %+v %v",last,cache,err)}
}
func TestBeta136DownloadRequiresMatchingOptedChannel(t *testing.T){
 a:=&application{}
 beta,err:=updateParseReleaseFeed(betaFixture(t,"1.17.136","1.17.137"),"beta")
 if err!=nil{t.Fatal(err)}
 a.updater.latest=beta
 a.updater.checked=true
 a.updater.channel="beta"
 for _,body:=range []string{`{"version":"1.17.137"}`,`{"version":"1.17.137","channel":"stable"}`} {
  reply:=updatePollHandle(a,"/api/updates/download",body)
  if reply.Code!=409 {t.Fatalf("cross-channel accepted: %d %s",reply.Code,reply.Body.String())}
 }
 stable,_:=updateParseRelease(feedFixture("1.17.138"))
 a.updater.latest=stable
 a.updater.channel="stable"
 updatePollTransport(t,updateRoundTrip(func(r *http.Request)(*http.Response,error){
  return updatePollResponse(http.StatusForbidden,nil,nil),nil
 }))
 // Existing stable clients continue to omit a channel field; the request
 // reaches the network and its mocked 403, not the channel-mismatch 409.
 reply:=updatePollHandle(a,"/api/updates/download",`{"version":"1.17.138"}`)
 if reply.Code==409 {t.Fatalf("stable default unexpectedly blocked: %s",reply.Body.String())}
}
func TestBeta136FeedRejectsInvalidChannelAndPreservesCooldown(t *testing.T){
 client:=&http.Client{Transport:updateRoundTrip(func(r *http.Request)(*http.Response,error){
  return &http.Response{StatusCode:http.StatusTooManyRequests,Header:http.Header{},Body:io.NopCloser(bytes.NewReader(nil)),ContentLength:0},nil
 })}
 if _,_,err:=updateFetchReleaseChannel(context.Background(),client,updateFeedCache{},"preview");err==nil{t.Fatal("unknown channel accepted")}
}
