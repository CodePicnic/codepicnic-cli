## Installation

###OS X:

Download and run the [OS X installer](https://apt.codepicnic.com/CodePicnic.pkg) .

###Ubuntu: 

Run this from your terminal

    wget -O- http://apt.codepicnic.com/codepicnic-cli-ubuntu.sh  | sh

## Configuration (set credentials)

Get your credentials (Client ID / Client Secret) from CodeGround.xyz https://codeground.xyz/dashboard/profile

### REPL Mode

Run this from your terminal

    codepicnic
    
Then inside the repl type 'configure':

    CodePicnic> configure
    
### CLI Mode

Run this from your terminal

    codepicnic configure

## Commands

All command run in CLI or REPL Mode. If you don't enter parameters, the program will ask for them.

* clear      clear screen
* configure  save configuration
* connect    connect to a console
* copy       copy a file from/to a console
* create     create and start a new console
* exit       exit the REPL
* help, h    Shows a list of commands or help for one command
* list       list consoles
* mount      mount /app filesystem from a container
* restart    restart a console
* start      start a console
* stop       stop a console
* unmount    unmount /app filesystem from a container
     

    
## Build instructions

    go build -ldflags "-X main.version=0.1 -X main.site=https://codeground.xyz -X main.swarm_host=tcp://54.88.32.109:4000" src/github.com/codepicnic/codepicnic-cli/codepicnic.go
