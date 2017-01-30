package openstack

import (
	"github.com/ghodss/yaml"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/rackspace/gophercloud/openstack/compute/v2/servers"
	"log"
	worker "tes/worker"
)

const startScript = `
#!/bin/sh
sudo systemctl start tes.service
`

func (s *scheduler) start(workerID string) {
	authOpts, aerr := openstack.AuthOptionsFromEnv()
	if aerr != nil {
		log.Printf("Auth options failed")
		log.Println(aerr)
		return
	}

	provider, perr := openstack.AuthenticatedClient(authOpts)
	if perr != nil {
		log.Printf("Provider failed")
		log.Println(perr)
		return
	}

	client, cerr := openstack.NewComputeV2(provider,
		gophercloud.EndpointOpts{Type: "compute", Name: "nova"})

	if cerr != nil {
		log.Printf("Provider failed")
		log.Println(cerr)
		return
	}

	// Write the worker config YAML file, which gets uploaded to the VM.
	workerConf := worker.Config{
		ID:            workerID,
		Timeout:       -1,
		ServerAddress: s.conf.ServerAddress,
		Storage:       s.conf.Storage,
	}
	workerConfYaml, _ := yaml.Marshal(workerConf)

	osconf := s.conf.Schedulers.Openstack
	_, serr := servers.Create(client, keypairs.CreateOptsExt{
		CreateOptsBuilder: servers.CreateOpts{
			Name:       osconf.Server.Name,
			FlavorName: osconf.Server.FlavorName,
			ImageName:  osconf.Server.ImageName,
			Networks:   osconf.Server.Networks,
			// Personality defines files that will be copied to the VM instance on boot.
			// We use this to upload TES worker config.
			Personality: []*servers.File{
				{
					Path:     osconf.ConfigPath,
					Contents: []byte(workerConfYaml),
				},
			},
			// Write a simple bash script that starts the TES service.
			// This will be run when the VM instance boots.
			UserData: []byte(startScript),
		},
		KeyName: osconf.KeyPair,
	}).Extract()

	if serr != nil {
		log.Printf("Error creating server")
		log.Println(serr)
	}
}
