// Copyright 2016-2018 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats-streaming/pb"
)

var usageStr = `
Usage: stan-sub [options] <subject>
Options:
	-s, --server   <url>            NATS Streaming server URL(s)
	-c, --cluster  <cluster name>   NATS Streaming cluster name
	-id,--clientid <client ID>      NATS Streaming client ID
Subscription Options:
	--qgroup <name>                 Queue group
	--seq <seqno>                   Start at seqno
	--all                           Deliver all available messages
	--last                          Deliver starting with last published message
	--since <duration>              Deliver messages in last interval (e.g. 1s, 1hr)
	         (for more information: https://golang.org/pkg/time/#ParseDuration)
	--durable <name>                Durable subscriber name
	--unsubscribe                   Unsubscribe the durable on exit
`

func usage() {
	log.Fatalf(usageStr)
}

func printMsg(m *stan.Msg, i int) {
	log.Printf("[#%d] Received on [%s]: '%v' '%v'\n", i, m.Subject, string(m.Data), m)
}

func printMsg2(m *stan.Msg, i int, e []byte) {
	log.Printf("Contains: %v - [#%d] Received on [%s]: '%v' '%v'\n", string(e), i, m.Subject, string(m.Data), m)
}

func main() {
	var clusterID string
	var clientID string
	var showTime bool
	var startSeq uint64
	var startDelta string
	var deliverAll bool
	var deliverLast bool
	var durable string
	var qgroup string
	var unsubscribe bool
	var URL string

	//	defaultID := fmt.Sprintf("client.%s", nuid.Next())
	// USE NUID
	// http://localhost:8222/streaming/channelsz?subs=1

	flag.StringVar(&URL, "s", stan.DefaultNatsURL, "The nats server URLs (separated by comma)")
	flag.StringVar(&URL, "server", stan.DefaultNatsURL, "The nats server URLs (separated by comma)")
	flag.StringVar(&clusterID, "c", "test-cluster", "The NATS Streaming cluster ID")
	flag.StringVar(&clusterID, "cluster", "test-cluster", "The NATS Streaming cluster ID")
	flag.StringVar(&clientID, "id", "test1", "The NATS Streaming client ID to connect with")
	flag.StringVar(&clientID, "clientid", "test1", "The NATS Streaming client ID to connect with")
	flag.BoolVar(&showTime, "t", true, "Display timestamps")
	// Subscription options
	flag.Uint64Var(&startSeq, "seq", 1, "Start at sequence no.")
	flag.BoolVar(&deliverAll, "all", false, "Deliver all")
	flag.BoolVar(&deliverLast, "last", false, "Start with last value")
	flag.StringVar(&startDelta, "since", "", "Deliver messages since specified time offset")
	flag.StringVar(&durable, "durable", "testdur", "Durable subscriber name")
	flag.StringVar(&qgroup, "qgroup", "testdurgroup", "Queue group name")
	flag.BoolVar(&unsubscribe, "unsubscribe", false, "Unsubscribe the durable on exit")

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	//args := flag.Args()

	if clientID == "" {
		log.Printf("Error: A unique client ID must be specified.")
		usage()
	}
	//if len(args) < 1 {
	//	log.Printf("Error: A subject must be specified.")
	//	usage()
	//}

	sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(URL),
		stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
			log.Fatalf("Connection lost, reason: %v", reason)
		}))
	if err != nil {
		log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s", err, URL)
	}
	log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", URL, clusterID, clientID)

	subj, i := "subject", 0

	mcb := func(msg *stan.Msg) {
		e := []byte("m")
		if bytes.Contains(msg.Data, e) {
			i++
			printMsg2(msg, i, e)
		} else {
			//i++
			//printMsg(msg, i)
		}
	}

	startOpt := stan.StartAt(pb.StartPosition_NewOnly)

	if startSeq != 0 {
		log.Println("start at", startSeq)
		startOpt = stan.StartAtSequence(startSeq)
	} else if deliverLast {
		startOpt = stan.StartWithLastReceived()
	} else if deliverAll {
		log.Print("subscribing with DeliverAllAvailable")
		startOpt = stan.DeliverAllAvailable()
	} else if startDelta != "" {
		ago, err := time.ParseDuration(startDelta)
		if err != nil {
			sc.Close()
			log.Fatal(err)
		}
		startOpt = stan.StartAtTimeDelta(ago)
	}

	start := time.Now()
	_, err = sc.QueueSubscribe(subj, qgroup, mcb, startOpt) // stan.DurableName(durable)
	if err != nil {
		sc.Close()
		log.Fatal(err)
	}
	log.Printf("%v", time.Since(start))

	log.Printf("Listening on [%s], clientID=[%s], qgroup=[%s] durable=[%s]\n", subj, clientID, qgroup, durable)

	if showTime {
		log.SetFlags(log.LstdFlags)
	}

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			sc.Close()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}
