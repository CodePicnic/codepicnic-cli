## Installation

On Ubuntu: 

    wget -O- http://apt.codepicnic.com/codepicnic-cli-ubuntu.sh  | sh
    
## Configuration (set credentials)


    codepicnic-cli configure 
    
## Build instructions

    go build -ldflags "-X main.version=0.1 -X main.site=https://codeground.xyz -X main.swarm_host=tcp://54.88.32.109:4000" src/github.com/codepicnic/codepicnic-cli/codepicnic.go
