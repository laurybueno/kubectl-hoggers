# kubectl-hoggers

## Development
```
docker build -t kubectl-hoggers:dev -f compose/local/Dockerfile .
docker run -it -v ${PWD}:/app kubectl-hoggers:dev bash
go install -v ./... && kubectl-hoggers
```
