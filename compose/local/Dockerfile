FROM golang:1.14

COPY compose/local/get_dependencies.sh /

WORKDIR /app

ENTRYPOINT [ "/get_dependencies.sh" ]

CMD [ "kubectl-hoggers" ]
