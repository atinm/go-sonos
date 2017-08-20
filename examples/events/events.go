//
// go-sonos
// ========
//
// Copyright (c) 2012-2015, Ian T. Richards <ianr@panix.com>
// Copyright (c) 2017, Atin M <atinm.dev@gmail.com>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
//   * Redistributions of source code must retain the above copyright notice,
//     this list of conditions and the following disclaimer.
//   * Redistributions in binary form must reproduce the above copyright
//     notice, this list of conditions and the following disclaimer in the
//     documentation and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED
// TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
package main

import (
	"encoding/xml"
	"log"
	"net"

	"github.com/atinm/go-sonos"
	"github.com/atinm/go-sonos/didl"
	"github.com/atinm/go-sonos/ssdp"
	"github.com/atinm/go-sonos/upnp"
)

const (
	EVENTS_PORT = "5007"
)

var (
	sonosDevices []*sonos.Sonos
)

// GetLocalInterfaceName returns the first interface name that has the non loopback local IPv4 addr of the host
func getLocalInterfaceName() string {
	list, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, iface := range list {
		addrs, err := iface.Addrs()
		if err != nil {
			panic(err)
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return iface.Name
				}
			}
		}
	}
	return ""
}

func getTriggeredSonos(svc *upnp.Service) (sonos *sonos.Sonos) {
	for _, s := range sonosDevices {
		if s.AVTransport.Svc == svc {
			sonos = s
			break
		}
	}
	return
}

func handleAVTransportEvents(reactor upnp.Reactor, c chan bool) {
	for {
		select {
		case evt := <-reactor.Channel():
			switch evt.Type() {
			case upnp.AVTransport_EventType:
				b := evt.(upnp.AVTransportEvent)

				log.Printf("[DEBUG] TransportState: %v", b.LastChange.InstanceID.TransportState.Val)
				log.Printf("[DEBUG] CurrentTrackURI: %v", b.LastChange.InstanceID.CurrentTrackURI.Val)
				log.Printf("[DEBUG] CurrentTrackMetadata: %s", b.LastChange.InstanceID.CurrentTrackMetaData.Val)
				if b.LastChange.InstanceID.CurrentTrackMetaData.Val != "" {
					var doc didl.Lite
					var artist, title, album string
					err := xml.Unmarshal([]byte(b.LastChange.InstanceID.CurrentTrackMetaData.Val), &doc)
					if err != nil {
						log.Panicf("Could not unmarshal %s: %v", b.LastChange.InstanceID.CurrentTrackMetaData.Val, err)
					}
					for _, item := range doc.Item {
						artist = item.Creator[0].Value
						album = item.Album[0].Value
						title = item.Title[0].Value
						log.Printf("[DEBUG] title: %s, artist: %s, album: %s", title, artist, album)
						break
					}
				} else {
					log.Printf("[DEBUG] CurrentTrackMetadata is empty: %s", b.LastChange.InstanceID.CurrentTrackMetaData.Val)
				}
				s := getTriggeredSonos(b.Svc)
				if s != nil {
					posInfo, err := s.GetPositionInfo(0)
					if nil != err {
						panic(err)
					}
					log.Printf("[DEBUG] Room Name: %s", s.Player.RoomName)
					log.Printf("[DEBUG] Position.TrackURI: %s", posInfo.TrackURI)
					log.Printf("[DEBUG] Position.TrackDuration: %s", posInfo.TrackDuration)
					log.Printf("[DEBUG] Position.RelTime: %s", posInfo.RelTime)

				}

				//s.Next(0)
			default:
				log.Panicf("[ERROR] Unexpected event %#v", evt)
			}
		}
	}
}

func SetupEvents(mgr ssdp.Manager) {
	// Startup and listen to events
	exit_chan := make(chan bool)
	reactor := sonos.MakeReactor(EVENTS_PORT)
	go handleAVTransportEvents(reactor, exit_chan)
	sonosDevices = sonos.ConnectAll(mgr, reactor, sonos.SVC_AV_TRANSPORT)
	<-exit_chan
}

// This code identifies UPnP devices on the netork that support the
// MusicServices API.
func main() {
	log.Print("go-sonos example events\n")

	mgr := ssdp.MakeManager()

	log.Printf("Discovering devices...")
	if err := mgr.Discover(getLocalInterfaceName(), EVENTS_PORT, false); nil != err {
		panic(err)
	} else {
		SetupEvents(mgr)
	}

	mgr.Close()
}
