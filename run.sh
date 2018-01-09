#! /bin/bash

# First compile the bchain application with 'go install'

# Run each of these aliases in separate terminals, starting with b4, then b3, b2, b1.

alias b1='bchain -idx=0 -addrs=:9080,:9081,:9082,:9083'
alias b2='bchain -idx=1 -addrs=:9080,:9081,:9082,:9083'
alias b3='bchain -idx=2 -addrs=:9080,:9081,:9082,:9083'
alias b4='bchain -idx=3 -addrs=:9080,:9081,:9082,:9083'
