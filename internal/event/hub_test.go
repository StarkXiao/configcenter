package event

import (
	"testing"
	"time"
)

func TestHubPublishesAndReplays(t *testing.T) {
	hub := NewHub(10)
	defer hub.Close()
	subscription, _, complete := hub.Subscribe("orders", "prod", "")
	defer subscription.Close()
	if !complete {
		t.Fatal("new subscription should be complete")
	}
	hub.Publish(Event{Application: "orders", Environment: "prod", Version: 1})
	var received Event
	select {
	case received = <-subscription.Events:
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}
	if received.Version != 1 || received.ID == "" {
		t.Fatalf("unexpected event: %+v", received)
	}
	replaySubscription, replay, complete := hub.Subscribe("orders", "prod", "0")
	defer replaySubscription.Close()
	if !complete || len(replay) != 1 || replay[0].Version != 1 {
		t.Fatalf("unexpected replay: complete=%v events=%+v", complete, replay)
	}
}

func TestHubSeparatesConfigurationSpaces(t *testing.T) {
	hub := NewHub(10)
	defer hub.Close()
	orders, _, _ := hub.Subscribe("orders", "prod", "")
	defer orders.Close()
	hub.Publish(Event{Application: "billing", Environment: "prod", Version: 1})
	select {
	case item := <-orders.Events:
		t.Fatalf("received event for another application: %+v", item)
	case <-time.After(20 * time.Millisecond):
	}
	if hub.Count("orders", "prod") != 1 {
		t.Fatal("subscription count changed unexpectedly")
	}
}
