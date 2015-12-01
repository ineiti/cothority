package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/crypto/abstract"
	"os"
	"strings"
)

func init() {
	command := cli.Command{
		Name:        "build",
		Aliases:     []string{"b"},
		Usage:       "Builds a cothority configuration file required for CoNodes and clients",
		Description: "Basically it will statically generate the tree, with the respective names and public key",
		ArgsUsage:   "HOSTFILE : file where each line is a copy of a public key node ( <address> <pubkey in b64> )",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "bf",
				Value: 2,
				Usage: "Defines the branching factor we want in the cothority tree. Default is 2",
			},
			cli.StringFlag{
				Name:  "config",
				Value: defaultConfigFile,
				Usage: "where to write the generated cothority configuration file",
			},
		},
		Action: func(c *cli.Context) {
			if c.Args().First() == "" {
				dbg.Fatal("You must provide a host file to generate the config")
			}
			Build(c.Args().First(), c.Int("bf"), c.String("config"))
		},
	}
	registerCommand(command)
}

// This file handles the creation a of cothority tree.
// Basically, it takes a list of files generated by the "key" command by each
// hosts and turn that into a full tree with the hostname and public key in each
// node.
// BuildTree takes a file formatted like this :
// host pubKey
// host2 pubKey
// ... ...
// For the moment it takes a branching factor on how to make the tree
// and the name of the file where to write the config
// It writes the tree + any other configs to output using toml format
// with the app/config_conode.go struct
func Build(hostFile string, bf int, configFile string) {

	// First, read the list of host and public keys
	hosts, pubs, err := readHostFile(hostFile)
	if err != nil {
		dbg.Fatal("Error reading the host file : ", err)
	}

	// Then construct the tree
	tree := constructTree(hosts, pubs, bf)
	// then constrcut the aggregated public key K0
	k0 := aggregateKeys(pubs)
	var b bytes.Buffer
	err = cliutils.WritePub64(suite, &b, k0)
	if err != nil {
		dbg.Fatal("Could not aggregate public keys in base64")
	}

	// Then write the config
	conf := app.ConfigConode{
		Suite:     suiteStr,
		Tree:      tree,
		Hosts:     hosts,
		AggPubKey: b.String(),
	}

	app.WriteTomlConfig(conf, configFile)
	dbg.Lvl1("Written config file with tree to", configFile)
}

// SImply adds all the public keys we give to it
func aggregateKeys(pubs []string) abstract.Point {
	k0 := suite.Point().Null()
	for i, ki := range pubs {
		// convert from string to public key
		kip, _ := cliutils.ReadPub64(suite, strings.NewReader(ki))
		k0 = k0.Add(k0, kip)
		dbg.Lvl2("Public key n#", i, ":", kip)
	}
	dbg.Lvl1("Aggregated public key:", k0)
	return k0
}

// readHostFile will read the host file
// HOSTNAME PUBLICKEY
// for each line. and returns the whole set and any errror if any are found.
func readHostFile(file string) ([]string, []string, error) {
	// open it up
	hostFile, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}

	// Then read it up
	hosts := make([]string, 0)
	pubs := make([]string, 0)
	scanner := bufio.NewScanner(hostFile)
	ln := 0
	for scanner.Scan() {
		line := scanner.Text()
		ln += 1
		spl := strings.Split(line, " ")
		if len(spl) != 2 {
			return nil, nil, errors.New(fmt.Sprintf("Hostfile misformatted at line %s", ln))
		}
		// add it HOSTS -> PUBLIC KEY
		h, err := cliutils.VerifyPort(spl[0], conode.DefaultPort)
		if err != nil {
			dbg.Fatal("Error reading address in host file :", spl[0], err)
		}
		hosts = append(hosts, h)
		pubs = append(pubs, spl[1])
	}
	dbg.Lvl1("Read the hosts files : ", ln, " entries")
	return hosts, pubs, nil
}

// ConstructTree takes a map of host -> public keys and a branching factor
// so it can constructs a regular tree. THe returned tree is the root
// it is constructed bfs style
func constructTree(hosts, pubs []string, bf int) *graphs.Tree {
	var root *graphs.Tree = new(graphs.Tree)
	root.Name = hosts[0]
	root.PubKey = pubs[0]
	var index int = 1
	bfs := make([]*graphs.Tree, 1)
	bfs[0] = root
	for len(bfs) > 0 && index < len(hosts) {
		t := bfs[0]
		t.Children = make([]*graphs.Tree, 0)
		lbf := 0
		// create space for enough children
		// init them
		for lbf < bf && index < len(hosts) {
			child := new(graphs.Tree)
			child.Name = hosts[index]
			child.PubKey = pubs[index]
			// append the children to the list of trees to visit
			bfs = append(bfs, child)
			t.Children = append(t.Children, child)
			index += 1
			lbf += 1
		}
		bfs = bfs[1:]
	}
	return root
}
