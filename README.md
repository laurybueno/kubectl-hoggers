# Analyze resouce consumption in Kubernetes from your terminal
Hoggers is a `kubectl` plugin that uses multiple Kubernetes API endpoints to show data about resource consumption in a Kubernetes cluster using only a terminal interface.

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
