package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/kataras/jev"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/clefclient"
	"github.com/rshade/go-decide/decision"
	"github.com/rshade/go-decide/jevclient"
)

const (
	backendJev  = "jev"
	backendClef = "clef"
)

// backend is one System One model the commands can ask. A run uses exactly
// one: its credentials, its error codes and its default thresholds.
type backend struct {
	name       string
	thresholds decision.Thresholds
	classify   func(context.Context, error) error
	newClient  func(env Env, cacheDir string) (*jev.Client, error)
}

// backends lists every backend, in the order the flag help names them. Both
// default thresholds are placeholders until #6 tunes each one.
var backends = []backend{
	{
		name:       backendJev,
		thresholds: decision.DefaultThresholds(),
		classify:   jevclient.Classify,
		newClient: func(env Env, cacheDir string) (*jev.Client, error) {
			var opts []jevclient.Option
			if cacheDir != "" {
				opts = append(opts, jevclient.WithResponseCache(cacheDir))
			}
			return env.NewClient(opts...)
		},
	},
	{
		name:       backendClef,
		thresholds: decision.DefaultThresholds(),
		classify:   clefclient.Classify,
		newClient: func(env Env, cacheDir string) (*jev.Client, error) {
			var opts []clefclient.Option
			if cacheDir != "" {
				opts = append(opts, clefclient.WithResponseCache(cacheDir))
			}
			return env.NewClefClient(opts...)
		},
	},
}

func backendNames() string {
	names := make([]string, len(backends))
	for i, b := range backends {
		names[i] = b.name
	}
	return strings.Join(names, ", ")
}

// backendFlag is the --backend flag shared by ask, score and eval.
type backendFlag struct {
	name string
}

func (f *backendFlag) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "backend", backendJev, "the System One model to ask: "+backendNames())
}

// resolve returns the chosen backend, or a validation failure that lists the
// allowed values. It runs before any request, and on a dry run.
func (f *backendFlag) resolve(ctx context.Context) (backend, error) {
	for _, b := range backends {
		if b.name == f.name {
			return b, nil
		}
	}
	return backend{}, newValidationError(ctx, fmt.Sprintf("--backend %q: must be one of %s", f.name, backendNames()), "backend")
}

// client builds the backend's client and classifies a construction failure
// with the backend's own codes.
func (b backend) client(ctx context.Context, env Env, cacheDir string) (*jev.Client, error) {
	client, err := b.newClient(env, cacheDir)
	if err != nil {
		return nil, b.classify(ctx, err)
	}
	return client, nil
}

func (b backend) decisionOptions(th decision.Thresholds) []decision.ChooseOption {
	return []decision.ChooseOption{decision.WithThresholds(th), decision.WithClassifier(b.classify)}
}
