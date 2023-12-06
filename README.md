# Features of the branch
This branch is based on the `add-volume` branch. It adds the following features:
- [x] Separate the read-from-fifo and write-to-fifo into two modules. Read-from-fifo is separated from `vk` and becomes the binary `fifo/read_from_fifo` whose source code is at `fifo/read_from_fifo.go`.
- [x] When env var `fifo` is set to `true`, the `vk` will use FIFO to execute the commands on the host from the container.
- [x] `fifo/read_from_fifo` should be run on the host before any submission of pods.