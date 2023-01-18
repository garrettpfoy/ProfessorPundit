FROM golang:latest

WORKDIR /app

COPY . .

RUN go get gopkg.in/yaml.v2
RUN go get github.com/bwmarrin/discordgo

RUN go build -o /professor-pundit

CMD ["/professor-pundit"]
