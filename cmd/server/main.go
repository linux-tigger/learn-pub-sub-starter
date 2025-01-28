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

func main() {
	fmt.Println("Starting Peril server...")
	// Create a channel to receive signals
	signalChan := make(chan os.Signal, 1)
	// Notify the channel on SIGINT (Ctrl+C) or SIGTERM
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	amqp_conn, _ := amqp.Dial(conn)
	mq_chan, _ := amqp_conn.Channel()
	err := pubsub.PublishJSON(mq_chan, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
	if err != nil {
		fmt.Println("Error publishing message:", err)
	}

	_, _, err = pubsub.DeclareAndBind(amqp_conn, routing.ExchangePerilDirect, "game_logs", "game_logs.*.", pubsub.Durable)
	if err != nil {
		fmt.Println("Error declaring and binding queue:", err)
	}

	// Run your application logic here
	go func() {
		fmt.Println("Program is running. Press Ctrl+C to exit...")
		gamelogic.PrintServerHelp()
		for {
			inputs := gamelogic.GetInput()
			if len(inputs) == 0 {
				continue
			}
			switch inputs[0] {
			case "pause":
				err := pubsub.PublishJSON(mq_chan, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
				if err != nil {
					fmt.Println("Pause Error!")
				}
			case "resume":
				err := pubsub.PublishJSON(mq_chan, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
				if err != nil {
					fmt.Println("Resume Error!")
				}
			case "quit":
				fmt.Println("Goodbye!")
				break
			default:
				fmt.Println("Unknown command. Type 'help' for a list of commands.")
			}
		}
	}()

	defer amqp_conn.Close()
	fmt.Println("Connection successful!")

	// Block until a signal is received
	sig := <-signalChan
	fmt.Printf("\nReceived signal: %s. Shutting down...\n", sig)

	// Perform cleanup operations here
	fmt.Println("Closing connections...")
	// Example: db.Close(), socket.Close(), etc.

	fmt.Println("Shutdown complete.")
}
