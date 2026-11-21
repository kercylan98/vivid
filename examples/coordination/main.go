package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/bootstrap"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (err error) {
	address := flag.String("address", "127.0.0.1:9101", "节点广告地址")
	path := flag.String("path", "node.wal", "本节点 WAL 路径，父目录须已存在")
	create := flag.Bool("create", false, "首次创建；恢复已有文件时不指定")
	flag.Parse()
	system := bootstrap.NewActorSystem(vivid.WithActorSystemRemoting(*address),
		vivid.WithActorSystemCoordination(vivid.ActorSystemCoordinationOptions{
			GroupID: "example", NodeID: 1, Members: map[uint64]string{1: *address}, Path: *path, Create: *create,
		}))
	if err = system.Start(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, system.Stop(5*time.Second)) }()
	status, err := system.CoordinationStatus(time.Second).Result()
	if err != nil {
		return err
	}
	fmt.Printf("group=%s node=%d term=%d leader=%d applied=%d\n", status.GroupID, status.NodeID, status.Term, status.Leader, status.Applied)
	return nil
}
