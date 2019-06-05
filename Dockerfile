# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
#FROM golang
FROM golang
ARG MAJOR_VER
# Copy the local package files to the container's workspace.
#ADD . /go/src/github.com/golang/example/outyet
#RUN go get github.com/DuoSoftware/DVP-DashboardDataProcessor/DashboardDataProcessor
#RUN go get gopkg.in/DuoSoftware/DVP-DashboardDataProcessor.$MAJOR_VER/DashboardDataProcessor
RUN go get -d -v github.com/DuoSoftware/DVP-DashboardDataProcessor/DashboardDataProcessor
# Build the outyet command inside the container.
# (You may fetch or manage dependencies here,
# either manually or with a tool like "godep".)
#RUN go install github.com/DuoSoftware/DVP-DashboardDataProcessor/DashboardDataProcessor
#RUN go install gopkg.in/DuoSoftware/DVP-DashboardDataProcessor.$MAJOR_VER/DashboardDataProcessor
RUN go install github.com/DuoSoftware/DVP-DashboardDataProcessor/DashboardDataProcessor

# Run the outyet command by default when the container starts.
ENTRYPOINT /go/bin/DashboardDataProcessor

# Document that the service listens on port 8840.
EXPOSE 8840
