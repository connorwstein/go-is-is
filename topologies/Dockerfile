FROM ubuntu:16.04
RUN apt-get update && apt-get install vim net-tools iputils-ping wget curl git build-essential gdb tcpdump unzip -y
RUN wget https://redirector.gvt1.com/edgedl/go/go1.9.2.linux-amd64.tar.gz 
RUN tar -C /usr/local -xzf go*.tar.gz
RUN echo "export PATH=$PATH:/usr/local/go/bin:/root/go/bin" >> /root/.bashrc
RUN echo "export TERM=xterm-256color" >> /root/.bashrc
RUN git clone https://github.com/connorwstein/dotfiles 
RUN cd dotfiles && ./setup
RUN curl -OL https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip
RUN unzip protoc-3.5.1-linux-x86_64.zip -d protoc3
RUN mv protoc3/bin/* /usr/local/bin/ 
RUN mv protoc3/include/* /usr/local/include/
RUN ln -s /usr/local/bin/protoc /usr/bin/protoc
RUN /usr/local/go/bin/go get -u google.golang.org/grpc 
RUN /usr/local/go/bin/go get -u github.com/golang/protobuf/protoc-gen-go
RUN /usr/local/go/bin/go get -u github.com/golang/glog
RUN /usr/local/go/bin/go get -u github.com/vishvananda/netlink
RUN /usr/local/go/bin/go get -u github.com/derekparker/delve/cmd/dlv
#RUN /usr/local/go/bin/go get -u github.com/connorwstein/go-is-is
RUN mkdir -p /root/go/src/github.com/connorwstein/go-is-is
WORKDIR /root/go/src/github.com/connorwstein/go-is-is
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
RUN curl -fsSL https://get.docker.com | sh
RUN echo 'set auto-load safe-path /' > /root/.gdbinit
CMD tail -f /dev/null
