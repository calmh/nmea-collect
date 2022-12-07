package main

import (
	"context"
	"fmt"
	"time"

	"github.com/BertoldVdb/go-ais"
	nmea "github.com/adrianmo/go-nmea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const contactRetention = 5 * time.Minute

var (
	aisContacts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "contacts_5min",
	}, []string{"class"})
)

type aisContactsCounter struct {
	c         <-chan string
	contactsA map[int32]time.Time
	contactsB map[int32]time.Time
}

func (l *aisContactsCounter) String() string {
	return fmt.Sprintf("ais-contacts-counter@%p", l)
}

func (l *aisContactsCounter) Serve(ctx context.Context) error {
	if l.contactsA == nil {
		l.contactsA = make(map[int32]time.Time)
	}
	if l.contactsB == nil {
		l.contactsB = make(map[int32]time.Time)
	}

	accountTicker := time.NewTicker(time.Minute)
	defer accountTicker.Stop()

	dec := ais.CodecNew(false, false)
	for {
		select {
		case line := <-l.c:
			sentence, err := nmea.Parse(line)
			if err != nil {
				continue
			}

			vdmvdo, ok := sentence.(nmea.VDMVDO)
			if !ok {
				continue
			}
			if vdmvdo.NumFragments > 1 {
				continue
			}

			pkt := dec.DecodePacket(vdmvdo.Payload)
			if pkt == nil {
				continue
			}

			hdr := pkt.GetHeader()
			switch hdr.MessageID {
			case 1, 2, 3: // Class A position report
				l.contactsA[int32(hdr.UserID)] = time.Now()
			case 18: // Class B position report
				l.contactsB[int32(hdr.UserID)] = time.Now()
			}
			l.account()

		case <-accountTicker.C:
			l.account()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (l *aisContactsCounter) account() {
	for k, v := range l.contactsA {
		if time.Since(v) > contactRetention {
			delete(l.contactsA, k)
		}
	}
	aisContacts.WithLabelValues("A").Set(float64(len(l.contactsA)))
	for k, v := range l.contactsB {
		if time.Since(v) > contactRetention {
			delete(l.contactsB, k)
		}
	}
	aisContacts.WithLabelValues("B").Set(float64(len(l.contactsB)))
}
