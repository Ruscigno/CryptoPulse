package stockscrapper

import "log"

// Start the server using channel to stop it
func Start() {
	server, err := NewServer()
	if err != nil {
		log.Fatal(err)
	}
	server.Start()

}
