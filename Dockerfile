FROM ubuntu:16.04
RUN apt-get update && apt-get install vim net-tools iputils-ping wget curl git build-essential -y
RUN wget https://redirector.gvt1.com/edgedl/go/go1.9.2.linux-amd64.tar.gz 
RUN tar -C /usr/local -xzf go*.tar.gz
RUN echo "export PATH=$PATH:/usr/local/go/bin" >> /root/.bashrc
RUN echo "export TERM=xterm-256color" >> /root/.bashrc
RUN git clone https://github.com/connorwstein/dotfiles 
RUN cd dotfiles && ./setup
RUN apt-get install tcpdump -y
WORKDIR /opt/go-is-is
CMD tail -f /dev/null
