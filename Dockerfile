FROM golang:1.17.1-alpine3.14 AS builder
RUN export GOPATH=/go
RUN apk update
RUN apk upgrade
RUN apk add --update gcc=10.3.1_git20210424-r2 g++=10.3.1_git20210424-r2
RUN apk --no-cache add   \
        git              
RUN mkdir -p /app
WORKDIR /app/
COPY go.* /app/
COPY *.go /app/
COPY models /app/models
COPY services /app/services
COPY utils /app/utils
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags '-linkmode external -w -extldflags "-static"'

FROM alpine:latest
ENV PRIVATE_CERT coiso
ENV PUBLIC_CERT coiso
RUN apk update
RUN apk upgrade
RUN apk add --no-cache 
RUN apk add --update --no-cache    \
		                 rsync

# Installing the openssh and bash package, removing the apk cache
RUN apk --update add --no-cache openssh bash \
  && sed -i s/#PermitRootLogin.*/PermitRootLogin\ yes/ /etc/ssh/sshd_config \
  && sed -i s/#StrictModes.*/StrictModes\ no/  /etc/ssh/sshd_config \
  && sed -i s/#PasswordAuthentication.*/PasswordAuthentication\ no/  /etc/ssh/sshd_config \
  && sed -i s/#PermitEmptyPasswords.*/PermitEmptyPasswords\ no/  /etc/ssh/sshd_config \
  && sed -i s/#ChallengeResponseAuthentication.*/ChallengeResponseAuthentication\ no/  /etc/ssh/sshd_config \
  # && sed -i s/#UsePAM.*/UsePAM\ no/  /etc/ssh/sshd_config \
  && sed -i s/#\ \ \ StrictHostKeyChecking.*/\ \ \ StrictHostKeyChecking\ no/  /etc/ssh/ssh_config \
  && rm -rf /var/cache/apk/*

# Defining the Port 22 for service
RUN sed -ie 's/#Port 22/Port 22/g' /etc/ssh/sshd_config
RUN /usr/bin/ssh-keygen -A
RUN ssh-keygen -t rsa -b 4096 -f  /etc/ssh/ssh_host_key
RUN mkdir -p /root/.ssh
RUN chmod 700 /root/.ssh
ENV NOTVISIBLE "in users profile"
RUN echo "export VISIBLE=now" >> /etc/profile
COPY --from=builder /app/local-storage-sync /bin/local-storage-sync
COPY ./run.sh /bin/run.sh
EXPOSE 22
RUN chmod 777 /bin/run.sh

CMD ["sh", "/bin/run.sh"]

