package main

import (
        "fmt"
        "time"
        "net/http"
        "encoding/json"
        "github.com/streadway/amqp"
)

// some very non-Golang-ish global variables
var identity string="WEB"
var fetcherkey string="FETCHERKEY"
var webkey="WEBKEY"
var myqueuename string="QWEB01"
var apicounter int=0
var EXCHANGE_NAME="exc"

type rmqdata struct{
        ResponseKey string `json:"responsekey"`
        MessageId string `json:"messageid"`
        Data []byte `json:"data"`
}

type dispenserdata struct{
        MessageId string
        ResponseChannel chan []byte
        Target string
        Data []byte
}

type subweather struct{
        Id int64                        `json:"id"`
        Main string                     `json:"main"`
        Description string              `json:"description"`
        Icon string                     `json:"icon"`
}

type weather struct{
        Coord map[string]float64        `json:"coord"`
        Weather []subweather            `json:"weather"`
        Base string                     `json:"base"`
        Main map[string]float64         `json:"main"`
        Visibility float64              `json:"visibility"`
        Wind map[string]float64         `json:"wind"`
        Clouds map[string]float64       `json:"clouds"`
        Datetime int64                  `json:"dt"`
        Sys map[string]interface{}      `json:"sys"`
        Id int64                        `json:"id"`
        Name string                     `json:"name"`
        Cod int64                       `json:"cod"`
}


// receive dispenserdata packets from the apihandlers and send them to the RMQ
func dispenser(rmqchannel *amqp.Channel, rmqrecvchannel <-chan amqp.Delivery, dispenserdatachannel chan dispenserdata){

        // create a cache of apihandlers and their channels
        apichanmap := make(map[string]chan []byte)

        for{
                select{
                        case datapkt := <-dispenserdatachannel:
                                // received a packet from web api; to be sent to RMQ
//                              fmt.Println("Received a packet on dispenser channel with msgid=", datapkt.MessageId)

                                // cache the response channel for the sender of this packet
                                apichanmap[datapkt.MessageId] = datapkt.ResponseChannel

                                rmqpktcontents,err:= json.Marshal(rmqdata{
                                                                ResponseKey:webkey,
                                                                MessageId:datapkt.MessageId,
                                                                Data:datapkt.Data})
                                if err!=nil{
                                        fmt.Println("Error Marshalling packet to send to RMQ=", err.Error())
                                        break
                                }
                                rmqmsg:=amqp.Publishing{
                                        DeliveryMode: amqp.Persistent,
                                        Type: "topic",
                                        Timestamp: time.Now(),
                                        ContentType: "text/plain",
                                        Body: rmqpktcontents,
                                }
                                if err:=rmqchannel.Publish(EXCHANGE_NAME, datapkt.Target, false, false, rmqmsg); err!=nil{
                                        fmt.Println("Error publishing message to RMQ=", err.Error())
                                        break
                                }
//                              fmt.Println("Message with id=", datapkt.MessageId," sent to RMQ for target=", datapkt.Target)

                        case datapkt:= <-rmqrecvchannel:
                                // packet is received from RMQ; to be sent to web api
                                datapkt.Ack(false)
//                              fmt.Println("Received a packet from RMQ bus")

                                rpkt := rmqdata{}
                                if err:= json.Unmarshal(datapkt.Body, &rpkt); err!=nil{
                                        fmt.Println("Error unmarshalling packet received from RMQ=", err.Error())
                                        fmt.Println("Packet data dump=", string(datapkt.Body))
                                        break
                                }
                                if _,ok:=apichanmap[rpkt.MessageId]; !ok{
                                        fmt.Println("Received packet with message id=", rpkt.MessageId, " has no cached receiver !")
                                        break
                                }
                                apichanmap[rpkt.MessageId]<- rpkt.Data
//                              fmt.Println("Sent response packet back to originator messageid=", rpkt.MessageId)
                }
        }
}

// Web API Handler function
func apihandler(w http.ResponseWriter, r *http.Request, dispenserdatachannel chan dispenserdata){

        myidentity:=fmt.Sprintf("%v-%v", identity, apicounter)
        apicounter++

        if err:=r.ParseForm(); err!=nil{
                fmt.Println("Error parsing URL's query params. Reason=", err.Error())
                fmt.Fprintf(w, "Error parsing URL query params=%v", err.Error())
                return
        }

        bodycontents, err:=json.Marshal(r.Form)
        if err!=nil{
                fmt.Println("Error JSON converting URL params to internal struct. Reason=", err.Error())
                fmt.Fprintf(w, "Error JSON converting URL params to internal struct. Reason=%v", err.Error())
                return
        }

        dmsg:=dispenserdata{
                MessageId: myidentity,
                ResponseChannel: make(chan []byte),
                Target: fetcherkey,
                Data: bodycontents,
        }

        // send this pkt to the dispenser routine
        dispenserdatachannel<- dmsg

        // wait to listen back on the ResponseChannel
        respdata:= <-dmsg.ResponseChannel

        resp := weather{}
        err=json.Unmarshal(respdata, &resp)
        if err!=nil{
                fmt.Println("Error in unmarshalling response data=", err.Error())
                fmt.Fprintf(w, "Internal processing error!")
                return
        }
        respstring:= "Weather in "+ resp.Name+ " is "+ resp.Weather[0].Description + "\n"
        fmt.Fprintf(w, respstring)

}

func main(){
        conn,err:=amqp.Dial("amqp://guest:guest@172.17.0.2:5672/")
        if err!=nil{
                fmt.Println("Error connecting to RMQ server=", err.Error())
                return
        }
        defer conn.Close()
        fmt.Println("Connected to RMQ !")

        rmqchannel, err:= conn.Channel()
        if err!=nil{
                fmt.Println("Error creating channel=", err.Error())
                return
        }
        defer rmqchannel.Close()
        fmt.Println("RMQ channel created !")

        if err:=rmqchannel.ExchangeDeclare(EXCHANGE_NAME, "topic", true, false, false, false, nil); err!=nil{
                fmt.Println("Exchange create failed due to=", err.Error())
                return
        }
        fmt.Println("RMQ exchange declaration done !")


        // declare and bind my queue
        _,err=rmqchannel.QueueDeclare(myqueuename, true, false, false, false, nil)
        if err!=nil{
                fmt.Println("Error declaring RMQ queue=", err.Error())
                return
        }
        err=rmqchannel.QueueBind(myqueuename, webkey, EXCHANGE_NAME, false, nil)
        if err!=nil{
                fmt.Println("Error binding my Queue to routing key=", err.Error())
                return
        }
        fmt.Println("Web queue declared and bound in RMQ!")

        // initiate consumption from the web queue
        rmqrecvchannel,err:=rmqchannel.Consume(myqueuename, identity, false, false, false, false, nil)
        if err!=nil{
                fmt.Println("Error scheduling consumption from Web queue in RMQ=", err.Error())
                return
        }

        // start the dispenser
        dispenserdatachannel := make(chan dispenserdata, 10)
        go dispenser(rmqchannel, rmqrecvchannel, dispenserdatachannel)

        http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request){
                                apihandler(w,r,dispenserdatachannel)
                        })
        http.ListenAndServe(":8881", nil)
}