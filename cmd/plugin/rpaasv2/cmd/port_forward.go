package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tsuru/rpaas-operator/internal/web/target"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type PortForward struct {
	Config          *rest.Config
	Clientset       kubernetes.Interface
	Name            string
	Labels          metav1.LabelSelector
	DestinationPort string
	ListenPort      int
	Namespace       string
	Ports           []string
	StopChan        chan struct{}
	ReadyChan       chan struct{}
}

type labelsFlags map[string]string

func (l *labelsFlags) String() string {
	return fmt.Sprintf("%v", *l)
}
func (l *labelsFlags) Set(value string) error {
	label := strings.SplitN(value, "=", 2)
	if len(label) != 2 {
		return errors.New("labels must include equal sign")
	}
	(*l)[label[0]] = label[1]
	return nil
}
func NewCmdPortForward() *cli.Command {
	return &cli.Command{
		Name:      "port-forward",
		Usage:     "",
		ArgsUsage: "[-p POD] [-l LOCALHOST] [-dp LOCAL_PORT:][-rl REMOTE_PORT]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "instance",
				Aliases: []string{"tsuru-instance", "i"},
				Usage:   "the Tsuru instance name",
			},
			&cli.StringFlag{
				Name:    "service",
				Aliases: []string{"tsuru-service", "s"},
				Usage:   "the Tsuru service name",
			},
			&cli.StringFlag{
				Name:    "pod",
				Aliases: []string{"p"},
				Usage:   "pod name - if omitted, the first pod will be chosen",
			},
			&cli.StringFlag{
				Name:    "address",
				Aliases: []string{"localhost", "l"},
				Usage:   "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, rpaas-operator will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.",
			},
			&cli.StringFlag{
				Name:    "destination-port",
				Aliases: []string{"dp"},
				Usage:   "specify a destined port",
			},
			&cli.StringFlag{
				Name:    "remote_port",
				Aliases: []string{"rp"},
				Usage:   "specify a remote port",
			},
		},
		Before: setupClient,
		Action: runPortForward,
	}
}

// func NewPortForwarder(namespace string, labels metav1.LabelSelector, port int) (*PortForward, error) {
// }

func runPortForward(c *cli.Context) error {
	var err error
	var Namespace, Pod string
	var ListenPort, Port int

	labels := labelsFlags{}
	args := rpaasclient.PortForwardArgs{
		Pod:             c.String("pod"),
		Address:         c.String("localhost"),
		Instance:        c.String("instance"),
		DestinationPort: c.Int("lp"),
		ListenPort:      c.Int("rp"),
	}

	flag.Var(&labels, "label", "")
	flag.IntVar(&ListenPort, "listen", ListenPort, "port to bind")
	flag.IntVar(&Port, "Port", args.DestinationPort, "port to forward")
	flag.StringVar(&Pod, "pod", args.Pod, "pod name")
	flag.StringVar(&Namespace, "namespace", args.Instance, "namespacepod look for")
	flag.Parse()

	pf, err := target.NewPortForwarder(Pod, metav1.LabelSelector{MatchLabels: labels}, Port, Namespace)
	if err != nil {
		return err
	}
	pf.ListenPort = ListenPort
	err = pf.Start(c.Context)
	if err != nil {
		log.Fatal("Error starting port forward:", err)
	}
	log.Printf("Started tunnel on %d\n", pf.ListenPort)
	time.Sleep(60 * time.Second)
	return nil
}
