// Package drivervbox implements a kutti driver for Oracle VirtualBox.
// It uses the VBoxManage tool to talk to VirtualBox.
//
// For cluster networking, it uses VirtualBox NAT networks. It allows
// port forwarding for host access to nodes.
// For nodes, it creates virtual machines by importing pre-packaged
// OVA files, maintained by the companion driver-vbox-images project.
// For images, it uses the aforesaid OVA files, downloading the list
// from the URL pointed to by the ImagesSourceURL variable.
//
// The details of individual operations can be found in the online
// documentation. Details about the interface between the driver and
// a running VM can be found at the driver-vbox-images project:
// https://github.com/kuttiproject/driver-vbox-images
package drivervbox
