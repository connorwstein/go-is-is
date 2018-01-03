FROM ubuntu:16.04
RUN apt-get update && apt-get install vim net-tools iputils-ping wget curl git build-essential -y
RUN wget https://redirector.gvt1.com/edgedl/go/go1.9.2.linux-amd64.tar.gz 
RUN tar -C /usr/local -xzf go*.tar.gz
RUN echo "export PATH=$PATH:/usr/local/go/bin:/root/go/bin" >> /root/.bashrc
RUN echo "export TERM=xterm-256color" >> /root/.bashrc
RUN git clone https://github.com/connorwstein/dotfiles 
RUN cd dotfiles && ./setup
RUN apt-get install tcpdump -y
RUN apt-get install unzip -y
RUN curl -OL https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip
RUN unzip protoc-3.5.1-linux-x86_64.zip -d protoc3
RUN mv protoc3/bin/* /usr/local/bin/ 
RUN mv protoc3/include/* /usr/local/include/
RUN ln -s /usr/local/bin/protoc /usr/bin/protoc
WORKDIR /opt/go-is-is
RUN /usr/local/go/bin/go get -u google.golang.org/grpc 
RUN /usr/local/go/bin/go get -u github.com/golang/protobuf/protoc-gen-go
CMD tail -f /dev/null
