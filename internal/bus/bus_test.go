package bus

import "testing"

func TestStickyAndFanout(t *testing.T) {
	b := New()
	b.PublishSticky("xray_status", "ok")
	b.Publish("event", 1) // not sticky, nobody listening yet
	ch1, cancel1 := b.Subscribe()
	if m := <-ch1; m.Type != "xray_status" || m.Data != "ok" {
		t.Fatalf("sticky not replayed: %+v", m)
	}
	ch2, cancel2 := b.Subscribe()
	<-ch2
	b.Publish("event", 2)
	if (<-ch1).Data != 2 || (<-ch2).Data != 2 {
		t.Fatal("fanout failed")
	}
	cancel1()
	b.Publish("event", 3)
	select {
	case m := <-ch1:
		t.Fatalf("unsubscribed channel received %+v", m)
	default:
	}
	cancel2()
	if m, ok := b.Last("xray_status"); !ok || m.Data != "ok" {
		t.Fatal("Last lost sticky value")
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	b := New()
	_, cancel := b.Subscribe()
	defer cancel()
	for i := 0; i < 1000; i++ {
		b.Publish("x", i)
	}
}
