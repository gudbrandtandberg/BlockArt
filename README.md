# BlockArt
Project 1 for CPSC416 Distributed Systems, UBC February 2018

This document explains how we have implemented the BlockArt project, which is specified at [http://www.cs.ubc.ca/~bestchai/teaching/cs416_2017w2/project1/index.html]

## Overview

The goal of the project is to implement a platform for a blockchain-based collaborative __art-project__.
The art project consists of a single global canvas, to which all participants can issue commands.
The main commands a user can issue to a canvas are __draw__ and __delete__. In addition, a user 
can perform a number of other queries to the canvas, including ink-level, blockchain-state and more. 

For a user to take part in the art-project, it must be connected to a single __miner__ in the 
__ink-mining__ network. A user connects to a miner using the miner's private key and the miner's network address. 
Miners are responsible for issuing commands on behalf of connected clients, as well as disseminating and validating
all operations on the distributed blockchain. 

![The network topology](html/img/Network.png])

## Graphics


## Webserver


## Mining



## Blockchain
