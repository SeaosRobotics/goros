package goros

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"golang.org/x/net/websocket"
)

var (
	messageCount = 0
)

type Base struct {
	Op string `json:"op"`
	Id string `json:"id"`
}

type Ros struct {
	origin           string
	url              string
	ws               *websocket.Conn
	receivedMapMutex sync.Mutex
	receivedMap      map[string]chan interface{}
}

func NewRos(url string) *Ros {
	ros := Ros{url: url, origin: "https://localhost"}
	ros.receivedMap = make(map[string]chan interface{})
	ros.connect()
	go ros.handleIncoming()
	return &ros
}

func (ros *Ros) connect() {
	ws, err := websocket.Dial(ros.url, "", ros.origin)
	if err != nil {
		log.Fatal(err)
	}

	ros.ws = ws
}

func (ros *Ros) getServiceResponse(service *ServiceCall) *ServiceResponse {
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[service.Id] = response
	ros.receivedMapMutex.Unlock()
	err := websocket.JSON.Send(ros.ws, service)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}

	serviceResponse := <-response
	return serviceResponse.(*ServiceResponse)
}

func (ros *Ros) getTopicResponse(topic *Topic) *interface{} {
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Id] = response
	ros.receivedMapMutex.Unlock()
	err := websocket.JSON.Send(ros.ws, topic)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}
	log.Println(ros.receivedMap)

	topicResponse := <-response
	return &topicResponse
}

func (ros *Ros) returnToAppropriateChannel(id string, data interface{}) {
	ros.receivedMapMutex.Lock()
	ros.receivedMap[id] <- data
	ros.receivedMapMutex.Unlock()
}

func (ros *Ros) handleIncoming() {
	var msg []byte
	for {
		err := websocket.Message.Receive(ros.ws, &msg)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Couldn't receive msg " + err.Error())
			break
		}

		/*
			opRegex, err := regexp.Compile(`"op"\s*:\s*"[[:alpha:],_]*`)
			if err != nil {
				log.Println(err)
			}
			opString := opRegex.FindString(string(msg))
			splitOpString := strings.Split(opString, "\"")
			operation := splitOpString[len(splitOpString)-1]
		*/

		var base Base
		json.Unmarshal(msg, &base)

		log.Println(base)

		if base.Op == "service_response" {
			var serviceResponse ServiceResponse
			json.Unmarshal(msg, &serviceResponse)
			ros.receivedMapMutex.Lock()
			ros.receivedMap[serviceResponse.Id] <- &serviceResponse
			ros.receivedMapMutex.Unlock()
		} else if base.Op == "publish" {
			log.Println(base)
			var topic Topic
			json.Unmarshal(msg, &topic)
			ros.receivedMapMutex.Lock()
			ros.receivedMap[topic.Topic] <- &topic
			ros.receivedMapMutex.Unlock()
		}
	}
}

func (ros *Ros) GetTopics() []string {
	response := ros.getServiceResponse(newServiceCall("/rosapi/topics"))
	var topics []string
	json.Unmarshal(response.Values["topics"], &topics)
	return topics
}

func (ros *Ros) GetServices() []string {
	response := ros.getServiceResponse(newServiceCall("/rosapi/services"))
	var services []string
	json.Unmarshal(response.Values["services"], &services)
	return services
}

func (ros *Ros) GetParams() []string {
        response := ros.getServiceResponse(newServiceCall("/rosapi/get_param_names"))
        var params []string
        json.Unmarshal(response.Values["names"], &params)
        return params
}

func (ros *Ros) Subscribe(topicName string, callback TopicCallback) {
	//topicResponse := ros.getTopicResponse(topic)
	topic := NewTopic(topicName)
	ros.SubscribeTopic(topic, callback)
}

func (ros *Ros) SubscribeTopic(topic *Topic, callback TopicCallback) {
	topic.Op = "subscribe"
	SetNewTopicId(topic)
	//log.Printf("DBG: topic : %v" , topic)
	//log.Printf("DBG: ros   : %v" , ros)
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Topic] = response
	ros.receivedMapMutex.Unlock()
	err := websocket.JSON.Send(ros.ws, *topic)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}

	go func() {
		for {
			callback(&(<-response).(*Topic).Msg)
		}
	}()
}

func (ros *Ros) SubscribeTopicWithChannel(topic *Topic, response *chan interface{}) {
	topic.Op = "subscribe"
	SetNewTopicId(topic)
	log.Printf("DBG: topic : %v" , topic)
	log.Printf("DBG: ros   : %v" , ros)
	//response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Topic] = *response
	ros.receivedMapMutex.Unlock()
	err := websocket.JSON.Send(ros.ws, *topic)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}
}

func (ros *Ros) OutboundTopic(topic *Topic) {
	SetNewTopicId(topic)
	log.Printf("DBG: topic : %v" , topic)
	log.Printf("DBG: ros   : %v" , ros)
	err := websocket.JSON.Send(ros.ws, *topic)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}
}

func (ros *Ros) AdvertiseTopic(topic *Topic) {
	topic.Op = "advertise"
	ros.OutboundTopic(topic)
}

func (ros *Ros) PublishTopic(topic *Topic) {
	topic.Op = "publish"
	ros.OutboundTopic(topic)
}


