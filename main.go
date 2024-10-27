/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	"github.com/jessevdk/go-flags"
	v1 "github.com/mackerelio-labs/mackerel-container-agent-sidecar-injector/api/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

type Config struct {
	MetricsAddr             string   `long:"metrics-bind-address" default:":8080" description:"The address the metric endpoint binds to."`
	ProveAddr               string   `long:"health-probe-bind-address" default:":8081" description:"The address the probe endpoint binds to."`
	EnableLeaderElection    bool     `long:"leader-elect" description:"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager."`
	AgentAPIKey             string   `long:"agentAPIKey" env:"SIDECAR_AGENT_API_KEY" description:"Mackerel API Key for the injected agent"`
	AgentKubeletPort        int      `long:"agentKubeletPort" env:"SIDECAR_AGENT_KUBELET_PORT" default:"-1" description:"Kubelet port"`
	AgentKubeletInsecureTLS bool     `long:"agentKubeletInsecureTLS" env:"SIDECAR_AGENT_KUBELET_INSECURE_PORT" description:"Skip verifying Kubelet host"`
	IgnoreNamespaces        []string `long:"ignoreNamespace" description:"Do not inject mackerel-container-agent into the Pod of the specified Namespaces."`
}

func main() {
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	var config Config
	parser := flags.NewParser(&config, flags.Default)
	if _, err := parser.Parse(); err != nil {
		setupLog.Error(err, "unable to parse config")
		os.Exit(1)
	}

	podWebHook := v1.NewPodWebHook()
	podWebHook.AgentAPIKey = config.AgentAPIKey
	podWebHook.AgentKubeletPort = config.AgentKubeletPort
	podWebHook.AgentKubeletInsecureTLS = config.AgentKubeletInsecureTLS
	podWebHook.IgnoreNamespaces = append(podWebHook.IgnoreNamespaces, config.IgnoreNamespaces...)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     config.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: config.ProveAddr,
		LeaderElection:         config.EnableLeaderElection,
		LeaderElectionID:       "75278439.mackerel.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = podWebHook.SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Pod")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
