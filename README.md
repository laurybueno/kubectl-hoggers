# Analyze resouce consumption in Kubernetes from your terminal
Hoggers is a `kubectl` plugin that uses multiple Kubernetes API endpoints to show data about resource consumption in a Kubernetes cluster using only a terminal interface.

## Installation
At the moment, only Linux distributions are supported. If you're running some other OS, try installing from source.

### From pre-built binary
This downloads the pre-built binary and moves it to your PATH.
```
curl -L -o kubectl-hoggers https://github.com/laurybueno/kubectl-hoggers/releases/download/v1.0.1/kubectl-hoggers
chmod +x kubectl-hoggers
sudo mv kubectl-hoggers /usr/bin
```

### From source
You need to have [Go installed](https://golang.org/doc/install) to build this application.
```
go get github.com/laurybueno/kubectl-hoggers
```

## Usage
This plugin uses the `KUBECONFIG` environment variable to access cluster data. It must be set for everything to work.

Check CPU and RAM reservations/limits for each node in a cluster
```
kubectl hoggers report
```

List pods consuming most CPU resources in real time along with its corresponding nodes. Refreshes every 10 seconds and requires metrics-server.
```
kubectl hoggers top
```

Note: because of the way `kubectl` plugins work, running `kubectl hoggers` or `kubectl-hoggers` gives the same results.

## Roadmap
- [ ] allow `KUBECONFIG` to be set via command flag
- [ ] output `report` to a file
- [ ] add a namespace option to `report`
- [ ] add table scroll in `report`
- [ ] option to order by RAM in `top`
- [ ] add animated GIFs to demonstrate usage

## Development workflow
I suggest using Docker for develping and build this plugin.

```
docker build -t kubectl-hoggers:dev -f compose/local/Dockerfile .
docker run -it --rm -v ${PWD}:/app kubectl-hoggers:dev bash
go install -v ./... && kubectl-hoggers
```
