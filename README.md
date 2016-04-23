# axis
etcd leader election for golang

# Usage

Create a configuration file name `axis.yaml`
```
etcd:
  endpoints:
    - http://127.0.0.1:2379
    - http://127.0.0.1:4001

daemon:
  prefix: /internal.network/services
  name: rethinkdb
  leader:
    entrypoint: rethinkdb
    command:
      - "--canonical-address"
      - "{{.AXIS_IP}}"
      - "--directory"
      - "/data"
      - "--bind"
      - "all"
    health:
      ports:
        - "28015/tcp"
        - "29015/tcp"
  worker:
    entrypoint: rethinkdb
    command:
      - "--canonical-address"
      - "{{.AXIS_IP}}"
      - "--directory"
      - "/data"
      - |
          {{range .AXIS_NODES}}
          --join {{.}}:29015
          {{end}}
      - "--bind"
      - "all"
    health:
      ports:
        - "28015/tcp"
        - "29015/tcp"
```
start your program with axis
```
$ axis axis.yaml
```

# How does this work?

this is an implementation of the ZooKeeper "recipe" 
[leader election](http://zookeeper.apache.org/doc/trunk/recipes.html#sc_leaderElection).


Let's say there are `NodeA`, `NodeB` and `NodeC`, `NodeA` finds lowest sorted createdIndex 
node and discovers that itself is the first node in the cluster, now `NodeA` is the leader 
node. `NodeB` and `NodeC` find the lowest sorted createdIndex node, find that `NodeA` 
is the leader, therefore start itself as a worker node.

```
NodeA -> /service/01
      ^
      |
    Watch (01)
      |
      |
NodeB -> /service/02
      ^
      |
    Watch (02)
      |
      |
NodeC -> /service/03
...
```

Now, `NodeA` crashes. `NodeA` node will be removed from etcd. `NodeB` and `NodeC` will get 
a notification of the leader change action. `NodeB` is now the lowest sorted index node, 
therefore `NodeB` becomes the leader node in the cluster. `NodeC` will now watch `NodeB`
instead of `NodeA`

```
NodeA -> /service/01
      ^
      |
    Watch (01)
      |
      |
NodeC -> /service/03
...
```

If some time later, `NodeA` rejoins, it would likely get `04` and be watching `NodeC`. 

## Documentation

`go doc` format documentation for this project can be viewed online without installing the package by using the GoDoc page at: https://godoc.org/github.com/samuelngs/axis

## Contributing

Everyone is encouraged to help improve this project. Here are a few ways you can help:

- [Report bugs](https://github.com/samuelngs/axis/issues)
- Fix bugs and [submit pull requests](https://github.com/samuelngs/axis/pulls)
- Write, clarify, or fix documentation
- Suggest or add new features

## License ##

This project is distributed under the MIT license found in the [LICENSE](./LICENSE)
file.

```
The MIT License (MIT)

Copyright (c) 2015 Samuel

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
