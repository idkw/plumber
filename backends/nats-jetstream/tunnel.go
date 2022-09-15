package nats_jetstream

import (
	"context"

	"github.com/pkg/errors"

	"github.com/batchcorp/plumber/validate"

	"github.com/batchcorp/plumber-schemas/build/go/protos/opts"
	"github.com/batchcorp/plumber-schemas/build/go/protos/records"
	"github.com/batchcorp/plumber/tunnel"
)

func (n *NatsJetstream) Tunnel(ctx context.Context, tunnelOpts *opts.TunnelOptions, tunnelSvc tunnel.ITunnel, errorCh chan<- *records.ErrorRecord) error {
	if err := validateTunnelOptions(tunnelOpts); err != nil {
		return errors.Wrap(err, "invalid tunnel options")
	}

	llog := n.log.WithField("pkg", "nats-jetstream/tunnel")

	if err := tunnelSvc.Start(ctx, "Nats", errorCh); err != nil {
		return errors.Wrap(err, "unable to create tunnel")
	}

	stream := tunnelOpts.NatsJetstream.Args.Subject

	outboundCh := tunnelSvc.Read()

	// Continually loop looking for messages on the channel.
	for {
		select {
		case outbound := <-outboundCh:
			if err := n.client.Publish(stream, outbound.Blob); err != nil {
				n.log.Errorf("Unable to replay message: %s", err)
				break
			}

			n.log.Debugf("Replayed message to Nats stream '%s' for replay '%s'", stream, outbound.ReplayId)
		case <-ctx.Done():
			llog.Debug("context cancelled")
			return nil
		}
	}

	return nil
}

func validateTunnelOptions(tunnelOpts *opts.TunnelOptions) error {
	if tunnelOpts == nil {
		return validate.ErrEmptyTunnelOpts
	}

	if tunnelOpts.NatsJetstream == nil {
		return validate.ErrEmptyBackendGroup
	}

	if tunnelOpts.NatsJetstream.Args == nil {
		return validate.ErrEmptyBackendArgs
	}

	if tunnelOpts.NatsJetstream.Args.Subject == "" {
		return ErrMissingSubject
	}

	return nil
}
