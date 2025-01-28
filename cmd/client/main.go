package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const conn = "amqp://guest:guest@localhost:5672/"

var glob_chan *amqp.Channel
var glob_conn *amqp.Connection

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		// if outcome == gamelogic.MoveOutComeSafe || outcome == gamelogic.MoveOutcomeMakeWar {
		// 	return pubsub.Ack
		// }
		if outcome == gamelogic.MoveOutComeSafe {
			return pubsub.Ack
		}
		if outcome == gamelogic.MoveOutcomeMakeWar {
			err := pubsub.PublishJSON(
				glob_chan,
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
		if outcome == gamelogic.MoveOutcomeSamePlayer {
			return pubsub.NackDiscard
		}
		return pubsub.NackDiscard
	}
}

func handlerWar(gs *gamelogic.GameState) func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
	//Consumes all the war messages that the "move" handler publishes,
	//no matter the username in the routing key.
	//Use a durable queue. The queue name should just be war. All clients will share this queue.
	//Whenever war is declared, only one client will consume the message.

	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, _, _ := gs.HandleWar(rw)
		if outcome == gamelogic.WarOutcomeNotInvolved {
			return pubsub.NackRequeue
		}
		if outcome == gamelogic.WarOutcomeNoUnits {
			return pubsub.NackDiscard
		}
		if outcome == gamelogic.WarOutcomeOpponentWon || outcome == gamelogic.WarOutcomeYouWon || outcome == gamelogic.WarOutcomeDraw {
			return pubsub.Ack
		}
		return pubsub.NackDiscard
	}
}

func main() {
	fmt.Println("Starting Peril client...")
	// Create a channel to receive signals
	signalChan := make(chan os.Signal, 1)
	// Notify the channel on SIGINT (Ctrl+C) or SIGTERM
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	amqp_conn, _ := amqp.Dial(conn)
	glob_conn = amqp_conn
	username, _ := gamelogic.ClientWelcome()

	defer amqp_conn.Close()

	// _, _, err := pubsub.DeclareAndBind(amqp_conn, routing.ExchangePerilDirect, pause_qname, routing.PauseKey, pubsub.Transient)
	// if err != nil {
	// 	fmt.Println("Error declaring and binding queue:", err)
	// }

	// Run your application logic here
	// Each game client should subscribe to moves from other players before starting its REPL.
	// Bind to the army_moves.* routing key.
	// Use army_moves.username as the queue name, where username is the name of the player.
	// Use the "peril_topic" exchange.
	// Use a transient queue
	// The handler for new messages should use the GameState's HandleMove method and then print a new > prompt for the user.
	go func() {
		mq_chan, _ := amqp_conn.Channel()
		glob_chan = mq_chan
		gamestate := gamelogic.NewGameState(username)
		moves_qname := fmt.Sprintf("army_moves.%s", username)
		pubsub.SubscribeJSON(amqp_conn, routing.ExchangePerilTopic, moves_qname, "army_moves.*", pubsub.Transient, handlerMove(gamestate))
		pause_qname := fmt.Sprintf("%s.%s", routing.PauseKey, username)
		pubsub.SubscribeJSON(amqp_conn, routing.ExchangePerilDirect, pause_qname, routing.PauseKey, pubsub.Transient, handlerPause(gamestate))
		pubsub.SubscribeJSON(glob_conn, routing.ExchangePerilTopic, "war", "war.*", pubsub.Durable, handlerWar(gamestate))
		for {
			inputs := gamelogic.GetInput()
			if len(inputs) == 0 {
				continue
			}
			switch inputs[0] {
			case "spawn":
				err := gamestate.CommandSpawn(inputs)
				if err != nil {
					fmt.Println("Spawn Error!")
				}
			case "move":
				user_move, err := gamestate.CommandMove(inputs)
				if err != nil {
					fmt.Println("Move Error!")
				}
				err = pubsub.PublishJSON(mq_chan, routing.ExchangePerilTopic, fmt.Sprintf("army_moves.%s", username), user_move)
				if err != nil {
					fmt.Println("Move Pub Error!")
				}
				fmt.Println("Move Successfully done and published!")
			case "status":
				gamestate.CommandStatus()
			case "spam":
				fmt.Println("Spamming not allowed yet!")
			case "help":
				gamelogic.PrintClientHelp()
			case "quit":
				gamelogic.PrintQuit()
				signalChan <- os.Interrupt
				break
			default:
				fmt.Println("Unknown command. Type 'help' for a list of commands.")
				continue
			}
		}
	}()

	// Block until a signal is received
	sig := <-signalChan
	fmt.Printf("\nReceived signal: %s. Shutting down...\n", sig)

	// Perform cleanup operations here
	fmt.Println("Closing connections...")
	// Example: db.Close(), socket.Close(), etc.

	fmt.Println("Shutdown complete.")
}
