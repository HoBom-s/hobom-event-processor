package main

import (
	"context"
	"log"
	"time"

	"github.com/HoBom-s/hobom-event-processor/internal/di"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		di.Module,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		log.Fatal(err)
	}

	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()

		if err := app.Stop(stopCtx); err != nil {
			log.Fatal(err)
		}
	}()
}
