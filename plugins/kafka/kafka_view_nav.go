package main

import (
	"omo/pkg/ui"
)

const (
	kafkaViewRoot       = "kafka"
	kafkaViewBrokers    = "brokers"
	kafkaViewTopics     = "topics"
	kafkaViewConsumers  = "consumers"
	kafkaViewPartitions = "partitions"
	kafkaViewMessages   = "messages"
)

func (kv *KafkaView) currentCores() *ui.CoreView {
	switch kv.currentView {
	case kafkaViewTopics:
		return kv.topicsView
	case kafkaViewConsumers:
		return kv.consumersView
	case kafkaViewPartitions:
		return kv.partitionsView
	case kafkaViewMessages:
		return kv.messagesView
	default:
		return kv.brokersView
	}
}

func (kv *KafkaView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{kafkaViewRoot, kafkaViewBrokers}
	if viewName != kafkaViewBrokers {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (kv *KafkaView) switchView(viewName string) {
	pageName := "kafka-" + viewName
	kv.currentView = viewName
	kv.viewPages.SwitchToPage(pageName)

	kv.setViewStack(kv.currentCores(), viewName)
	kv.refresh()
	current := kv.currentCores()
	if current != nil {
		kv.app.SetFocus(current.GetTable())
	}
}

func (kv *KafkaView) showBrokers() {
	kv.switchView(kafkaViewBrokers)
}

func (kv *KafkaView) showTopics() {
	kv.switchView(kafkaViewTopics)
}

func (kv *KafkaView) showConsumers() {
	kv.switchView(kafkaViewConsumers)
}

func (kv *KafkaView) showPartitions() {
	kv.switchView(kafkaViewPartitions)
}

func (kv *KafkaView) showMessages() {
	kv.switchView(kafkaViewMessages)
}
