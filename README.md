# ProfessorPundit
Discord Bot built for UIC-Trackers discord that fetches and sorts professor's https://www.ratemyprofessors.com ratings.

## Installation
This bot was written in Go, but developed using Docker, thus the recommended installation method is Docker. Either through Docker run, or docker-compose.

I personally prefer docker-compose, especially since it is clone-and-go, so that will be what I am covering in this readme.

### Install Docker
Before anything, ensure you have (Docker)[https://www.docker.com] installed. If you wish to use docker-compose, ensure it is downloaded as well.

### Build Container
```
docker-compose up --build [-d | Detached] pundit
```

### Remove Container
```
docker-compose rm -vf pundit
```

### Get logs of container
```
docker-compose logs [-f | Follow] pundit
```

### Stop Container
```
docker-compose down -v pundit
```
