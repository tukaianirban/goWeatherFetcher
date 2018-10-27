package main

import (
        "fmt"
        "sync"
        "time"
        "encoding/json"
        "net/http"
        "net/url"
        "io/ioutil"
        "github.com/streadway/amqp"
)

// global variables
var identity string="FETCHER"
var fetcherkey string="FETCHERKEY"
var webkey string="WEBKEY"
var myqueuename string="QFETCHER01"
var NUM_WORKERS=5
var EXCHANGE_NAME="exc"

var weatherapiurl string="http://api.openweathermap.org/data/2.5/weather?APPID=964d0e778fe8dd987975f2f9de7b3f70"


type rmqdata struct{
        MessageId string
        ResponseKey string
        Data []byte
}

// method to start off the worker routines
func Worker(workerid int, workerdatachannel chan rmqdata, workerrespchannel chan rmqdata){

        LOGSTR:="Worker(" + fmt.Sprintf("%v",workerid) + "):"

        client:= http.Client{}

        for data:= range workerdatachannel{
//              fmt.Println("Worker id=", workerid, " received a packet with id=", data.MessageId)

                // JSON Unmarshal the url.Values that came in as Data
                datavalues:=url.Values{}
                if err:=json.Unmarshal(data.Data, &datavalues); err!=nil{
                        fmt.Println("Unable to JSON unmarshal the contents of the user request. Error=", err.Error())
                        continue
                }
                u,_ := url.Parse(weatherapiurl)
                queryparams := u.Query()
                if loc,ok:=datavalues["location"]; !ok{
                        fmt.Println(LOGSTR,"Could not find 'location' key in user request")
                        continue
                }else{
                        queryparams.Set("q", loc[0])
                }
                u.RawQuery = queryparams.Encode()
                req, err:=http.NewRequest("GET", u.String(), nil)
                if err!=nil{
                        fmt.Println(LOGSTR, "Failed to create HTTP Request. Reason=", err.Error())
                        continue
                }
                resp,err:= client.Do(req)
                if err!=nil{
                        fmt.Println("Error in HTTP Response=", err.Error())
                        continue
                }
                respbody, err:=ioutil.ReadAll(resp.Body)
                resp.Body.Close()
                if err!=nil{
                        fmt.Println(LOGSTR,"Error reading Response body=", err.Error())
                        continue
                }
                workerrespchannel<- rmqdata{
                        ResponseKey:data.ResponseKey,
                        MessageId:data.MessageId,
                        Data:respbody,
                }
//              fmt.Println(LOGSTR,"Sent response back to messageid=", data.MessageId)
        }
}


// goroutine to send the responses back to the RMQ
func responder(rmqchannel *amqp.Channel, datachannel chan rmqdata){

        for data := range datachannel{

                datacontents,err:=json.Marshal(data)
                if err!=nil{
                        fmt.Println("Error converting response data to byte slice=", err.Error())
                        continue
                }
                msg := amqp.Publishing{
                        DeliveryMode: amqp.Persistent,
                        Type: "topic",
                        Timestamp: time.Now(),
                        ContentType: "text/plain",
                        Body: datacontents,
                }
                if err:=rmqchannel.Publish(EXCHANGE_NAME, data.ResponseKey, false, false, msg); err!=nil{
                        fmt.Println("Error publishing response message to RMQ=", err.Error())
                }
        }
}


// read from the rmq channel, json unmarshal it, dispatch to worker thread's channel
func dispatcher(datachannel <-chan amqp.Delivery, workerdatachannel chan rmqdata){


        for msg:=range datachannel{
                msg.Ack(false)

                // json unmarshal it
                datapkt:=rmqdata{}
                err:=json.Unmarshal(msg.Body, &datapkt)
                if err!=nil{
                        fmt.Println("Error JSON-Unmarshalling the recvd packet=", err.Error())
                        continue
                }
                workerdatachannel<- datapkt
        }


}



func main(){

        conn,err := amqp.Dial("amqp://guest:guest@172.17.0.2:5672/")
        if err!=nil{
                fmt.Println("Error connecting to RMQ server=", err.Error())
                return
        }
        defer conn.Close()
        fmt.Println("Connected to RMQ")

        rmqchannel, err:=conn.Channel()
        if err!=nil{
                fmt.Println("Error creating channel in RMQ=", err.Error())
                return
        }
        defer rmqchannel.Close()
        fmt.Println("Channel created in RMQ")

        if err:=rmqchannel.ExchangeDeclare(EXCHANGE_NAME, "topic", true, false, false, false, nil); err!=nil{
                fmt.Println("Error declaring the Exchange in RMQ=", err.Error())
                return
        }
        fmt.Println("Exchange declared in RMQ")

        _,err=rmqchannel.QueueDeclare(myqueuename, true, false, false, false, nil)
        if err!=nil{
                fmt.Println("Error declaring RMQ queue=", err.Error())
                return
        }
        err=rmqchannel.QueueBind(myqueuename, fetcherkey, EXCHANGE_NAME, false, nil)
        if err!=nil{
                fmt.Println("Error binding my Queue to routing key=", err.Error())
                return
        }
        fmt.Println("Fetcher queue declared and bound in RMQ!")

        // enable consumption
        rmqrecvchannel, err:=rmqchannel.Consume(myqueuename, identity, false, false, false, false, nil)
        if err!=nil{
                fmt.Println("Data channel consumption could not be started=", err.Error())
                return
        }

        // dispatcher routine reads from RMQ bus and feeds into worker routines
        workersdatachannel := make(chan rmqdata,NUM_WORKERS*2)
        workersrespchannel := make(chan rmqdata, NUM_WORKERS*2)

        wg := sync.WaitGroup{}

        go dispatcher(rmqrecvchannel, workersdatachannel)
        wg.Add(1)

        for i:=0;i<NUM_WORKERS;i++{
                go Worker(i, workersdatachannel, workersrespchannel)
//              fmt.Println("Started worker#", i)
        }

        go responder(rmqchannel, workersrespchannel)
        wg.Add(1)

        wg.Wait()

}