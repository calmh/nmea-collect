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

var (
	aisContacts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "contacts_5min",
	}, []string{"class"})
)

type aisContactsCounter struct {
	c <-chan string
}

func (l *aisContactsCounter) String() string {
	return fmt.Sprintf("ais-contacts-counter@%p", l)
}

func (l *aisContactsCounter) Serve(ctx context.Context) error {
	const contactRetention = 5 * time.Minute

	contactsA := make(map[int32]time.Time)
	contactsB := make(map[int32]time.Time)
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
				contactsA[int32(hdr.UserID)] = time.Now()
				for k, v := range contactsA {
					if time.Since(v) > contactRetention {
						delete(contactsA, k)
					}
				}
				aisContacts.WithLabelValues("A").Set(float64(len(contactsA)))
			case 18: // Class B position report
				contactsB[int32(hdr.UserID)] = time.Now()
				for k, v := range contactsB {
					if time.Since(v) > contactRetention {
						delete(contactsB, k)
					}
				}
				aisContacts.WithLabelValues("B").Set(float64(len(contactsB)))
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
