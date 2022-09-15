package mqtt

import (
	"context"
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"

	"github.com/batchcorp/plumber-schemas/build/go/protos/opts"
	"github.com/batchcorp/plumber-schemas/build/go/protos/records"

	"github.com/batchcorp/plumber/validate"
)

func (m *MQTT) Read(ctx context.Context, readOpts *opts.ReadOptions, resultsChan chan *records.ReadRecord, errorChan chan *records.ErrorRecord) error {
	if err := validateReadOptions(readOpts); err != nil {
		return errors.Wrap(err, "unable to validate read options")
	}

	var count int64
	doneCh := make(chan struct{}, 1)

	var readFunc = func(client mqtt.Client, msg mqtt.Message) {
		count++

		serializedMsg, err := json.Marshal(msg)
		if err != nil {
			errorChan <- &records.ErrorRecord{
				OccurredAtUnixTsUtc: time.Now().UTC().Unix(),
				Error:               errors.Wrap(err, "unable to serialize message into JSON").Error(),
			}
		}

		t := time.Now().UTC().Unix()

		resultsChan <- &records.ReadRecord{
			MessageId:           uuid.NewV4().String(),
			Num:                 count,
			ReceivedAtUnixTsUtc: t,
			Payload:             msg.Payload(),
			XRaw:                serializedMsg,
			Record: &records.ReadRecord_Mqtt{
				Mqtt: &records.MQTT{
					Id:        uint32(msg.MessageID()),
					Topic:     msg.Topic(),
					Value:     msg.Payload(),
					Duplicate: msg.Duplicate(),
					Retained:  msg.Retained(),
					//Qos:       msg.Qos(), TODO: how to convert []byte to uint32
					Timestamp: t,
				},
			},
		}

		if !readOpts.Continuous {
			doneCh <- struct{}{}
		}
	}

	m.log.Info("Listening for messages...")

	token := m.client.Subscribe(readOpts.Mqtt.Args.Topic, byte(m.connArgs.QosLevel), readFunc)
	if err := token.Error(); err != nil {
		return err
	}

	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return nil
	}

	return nil
}

func validateReadOptions(readOpts *opts.ReadOptions) error {
	if readOpts == nil {
		return validate.ErrMissingReadOptions
	}

	if readOpts.Mqtt == nil {
		return validate.ErrEmptyBackendGroup
	}

	args := readOpts.Mqtt.Args
	if args == nil {
		return validate.ErrEmptyBackendArgs
	}

	if args.Topic == "" {
		return ErrEmptyTopic
	}

	return nil
}
