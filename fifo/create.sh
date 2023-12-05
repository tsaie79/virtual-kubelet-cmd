#!/bin/bash
# run this script on host machine!

export fifo_path=/home/jeng-yuantsai/Documents/JIRIAF/virtual-kubelet-cmd/fifo

# if folder $HOME/hostpip exists, do nothing; otherwise, create it
if [ ! -d "$fifo_path/hostpipe" ]; then
    mkdir $fifo_path/hostpipe
fi

# if file $HOME/hostpipe/vk-cmd exists, do nothing; otherwise, create it
if [ ! -p "$fifo_path/hostpipe/vk-cmd" ]; then
    mkfifo $fifo_path/hostpipe/vk-cmd

fi

# while true do eval "$(cat $HOME/hostpipe/vk-cmd)" save stderror and stdout to diferent files
while true; do
    eval "$(cat $fifo_path/hostpipe/vk-cmd)" > $fifo_path/hostpipe/pipeline.out 2> $fifo_path/hostpipe/pipeline.err
done