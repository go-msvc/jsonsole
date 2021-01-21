//Package main is a terminal program to run an app session like USSD
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"github.com/go-msvc/errors"
	japp "github.com/go-msvc/japp/msg"
	jclihttp "github.com/go-msvc/jcli/http"
)

func main() {
	appURL := "http://localhost:12345/app"
	cli, err := jclihttp.New(appURL)
	if err != nil {
		panic(errors.Wrapf(err, "failed to create client"))
	}

	//start session in app
	//loop to communicate with app
	res, err := cli.Call("start", japp.StartRequest{}, reflect.TypeOf(japp.StartResponse{}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED to start session: %+v\n", err)
		os.Exit(1)
	}
	startRes := res.(japp.StartResponse)

	//create a user input channel used for all console input
	//so we can constantly read the terminal
	userInputChan := make(chan string)
	go func(userInputChan chan string) {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, _ := reader.ReadString('\n')
			input = strings.Replace(input, "\n", "", -1)
			userInputChan <- input
		}
	}(userInputChan)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT) //<ctrl><C>
	go func() {
		<-signalChannel
		userInputChan <- "exit"
	}()

	fmt.Fprintf(os.Stdout, "\n")
	fmt.Fprintf(os.Stdout, "===== J - C O N S O L E ===============\n")

	//main loop
	lastContent := startRes.Content
	for {
		//show content
		fmt.Fprintf(os.Stdout, "---------------------------------------\n")
		if err := renderContent(os.Stdout, lastContent); err != nil {
			fmt.Fprintf(os.Stderr, "failed to render: %+v\n", err)
			os.Exit(1)
		}

		if lastContent.Final {
			fmt.Fprintf(os.Stdout, "\nDone.\n")
			os.Exit(0)
		}

		//wait for next user input
		input := ""
		for len(input) == 0 {
			fmt.Fprintf(os.Stdout, "J-Console> ")
			input = <-userInputChan
		}
		if input == "exit" {
			fmt.Fprintf(os.Stdout, "Terminated.\n")
			break
		}

		//send input to app
		contReq := japp.ContinueRequest{
			SessionID: startRes.SessionID,
			StepID:    lastContent.StepID,
			Data:      map[string]interface{}{"input": input},
		}
		res, err := cli.Call("cont", contReq, reflect.TypeOf(japp.ContinueResponse{}))
		if err != nil {
			fmt.Printf("Failed to continue app: %+v", err)
			continue
		}
		contRes := res.(japp.ContinueResponse)
		lastContent = contRes.Content
	} //for main loop
} //main()

func renderContent(w io.Writer, c japp.Content) error {
	if m := c.Message; m != nil {
		fmt.Fprintf(w, "Message: %s\n", m.Text)
		return nil
	}
	if p := c.Prompt; p != nil {
		fmt.Fprintf(w, "Prompt: %s\n", p.Text)
		return nil
	}
	if c := c.Choice; c != nil {
		fmt.Fprintf(w, "Choice: %s\n", c.Header)
		for _, o := range c.Options {
			fmt.Fprintf(w, "%s) %s\n", o.ID, o.Text)
		}
		return nil
	}

	//unexpected type of content
	return errors.Errorf("Cannot render content: %+v", c)
}
