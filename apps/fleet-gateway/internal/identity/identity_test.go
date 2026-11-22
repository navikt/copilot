package identity

import (
	"errors"
	"reflect"
	"testing"
)

var _ Resolver[Session] = SessionResolver{}

func TestSessionResolver(t *testing.T) {
	got, err := SessionResolver{}.Resolve(Session{NAVIdent: "Z123456", ClientID: "https://c.example/client.json"})
	want := Caller{Issuer: "fleet-gateway", Subject: "Z123456", Actor: "https://c.example/client.json",
		ActorType: "oauth_client", Chain: []string{"https://c.example/client.json"}, Assurance: "session"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

// Contract every resolver keeps: no caller without both subject and actor.
func TestSessionResolver_FailsClosed(t *testing.T) {
	for in, wantErr := range map[Session]error{
		{ClientID: "c"}:  ErrNoSubject,
		{NAVIdent: "Z1"}: ErrNoActor,
		{}:               ErrNoSubject,
	} {
		got, err := SessionResolver{}.Resolve(in)
		if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, Caller{}) {
			t.Errorf("%+v: got %+v, %v; want %v", in, got, err, wantErr)
		}
	}
}
