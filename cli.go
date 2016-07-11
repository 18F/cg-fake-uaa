package main

import (
	"flag"
	"fmt"
	"github.com/fatih/color"
	"net/http"
)

func main() {
	var callbackUrl string

	cyan := color.New(color.FgCyan).SprintFunc()

	portPtr := flag.Int("port", 8080, "Port to listen on")
	flag.StringVar(&callbackUrl, "callback-url", "http://localhost:8000/auth/callback", "OAuth2 Callback URL")

	flagNoColor := flag.Bool("no-color", false, "Disable color output")

	flag.Parse()

	if *flagNoColor {
		color.NoColor = true
	}

	handler := NewServerHandler(&ServerConfig{
		CallbackUrl: Urlify(callbackUrl),
	})

	authorizeUrl := fmt.Sprintf("http://localhost:%d%s", *portPtr, Urls.Reverse("authorize"))
	tokenUrl := fmt.Sprintf("http://localhost:%d%s", *portPtr, Urls.Reverse("token"))

	fmt.Fprintf(color.Output, "My OAuth2 authorize URL is %s.\n", cyan(authorizeUrl))
	fmt.Fprintf(color.Output, "My OAuth2 token URL is %s.\n", cyan(tokenUrl))
	fmt.Fprintf(color.Output, "Your client's callback URL is %s.\n", cyan(callbackUrl))
	fmt.Fprintf(color.Output, "To change settings, call me with the -help flag.\n\n")

	fmt.Fprintf(color.Output, "Starting fake-cloud.gov server on port %s.\n", cyan(*portPtr))

	http.HandleFunc("/", handler)
	http.ListenAndServe(fmt.Sprintf(":%d", *portPtr), nil)
}
