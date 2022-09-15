package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"

	"github.com/batchcorp/plumber-schemas/build/go/protos/opts"
	"github.com/batchcorp/plumber-schemas/build/go/protos/records"

	"github.com/batchcorp/plumber/validate"
)

func (n *Nats) Read(ctx context.Context, readOpts *opts.ReadOptions, resultsChan chan *records.ReadRecord, errorChan chan *records.ErrorRecord) error {
	if err := validateReadOptions(readOpts); err != nil {
		return errors.Wrap(err, "unable to validate read options")
	}

	n.log.Info("Listening for message(s) ...")

	var count int64

	// nats.Subscribe is async, use channel to wait to exit
	doneCh := make(chan struct{})
	defer close(doneCh)

	n.Client.Subscribe(readOpts.Nats.Args.Subject, func(msg *nats.Msg) {
		count++

		serializedMsg, err := json.Marshal(msg)
		if err != nil {
			errorChan <- &records.ErrorRecord{
				OccurredAtUnixTsUtc: time.Now().UTC().Unix(),
				Error:               errors.Wrap(err, "unable to serialize message into JSON").Error(),
			}
			return
		}

		resultsChan <- &records.ReadRecord{
			MessageId:           uuid.NewV4().String(),
			Num:                 count,
			ReceivedAtUnixTsUtc: time.Now().UTC().Unix(),
			Payload:             msg.Data,
			XRaw:                serializedMsg,
			Record: &records.ReadRecord_Nats{
				Nats: &records.Nats{
					Subject: msg.Subject,
					Value:   msg.Data,
				},
			},
		}

		if !readOpts.Continuous {
			doneCh <- struct{}{}
		}
	})

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
		return errors.New("read options cannot be nil")
	}

	if readOpts.Nats == nil {
		return validate.ErrEmptyBackendGroup
	}

	if readOpts.Nats.Args == nil {
		return validate.ErrEmptyBackendArgs
	}

	if readOpts.Nats.Args.Subject == "" {
		return ErrMissingSubject
	}

	return nil
}
