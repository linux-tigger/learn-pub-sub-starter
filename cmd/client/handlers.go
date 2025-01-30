package main

import (
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerMove(gs *gamelogic.GameState, publishCh *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")

		moveOutcome := gs.HandleMove(move)
		switch moveOutcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.Ack
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				publishCh,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gs.GetUsername(),
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.GetPlayerSnap(),
				},
			)
			if err != nil {
				fmt.Printf("error: %s\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		}

		fmt.Println("error: unknown move outcome")
		return pubsub.NackDiscard
	}
}

// Update the war handler function in the client to publish game logs. All the logs should use:
// The topic exchange
// The GameLogSlug.username routing key, where username is the name of the player who initiated the war, and GameLogSlug is a constant in the routing package.
// The GameLog struct should be serialized using the PublishGob function. Fill all the fields in.
func handlerWar(gs *gamelogic.GameState, publishCh *amqp.Channel) func(dw gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(dw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		warOutcome, winner, loser := gs.HandleWar(dw)

		switch warOutcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			pubsub.PublishGob(
				publishCh,
				routing.ExchangePerilTopic,
				routing.GameLogSlug+"."+gs.GetUsername(),
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
					Username:    gs.GetUsername(),
				},
			)
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			err := pubsub.PublishGob(
				publishCh,
				routing.ExchangePerilTopic,
				routing.GameLogSlug+"."+gs.GetUsername(),
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
					Username:    gs.GetUsername(),
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			err := pubsub.PublishGob(
				publishCh,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gs.GetUsername(),
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser),
					Username:    gs.GetUsername(),
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		}

		fmt.Println("error: unknown war outcome")
		return pubsub.NackDiscard
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}
