// Copyright 2014-2017 Bret Jordan, All rights reserved.
//
// Use of this source code is governed by an Apache 2.0 license
// that can be found in the LICENSE file in the root of the source
// tree.

package main

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"github.com/jordan2175/rewritecap/lib/arp"
	"github.com/jordan2175/rewritecap/lib/header"
	"github.com/jordan2175/rewritecap/lib/layer2"
	"github.com/jordan2175/rewritecap/lib/layer3"
	"github.com/pborman/getopt"
	"os"
	"strings"
)

var sOptPcapSrcFilename = getopt.StringLong("file", 'f', "", "Filename of the source PCAP file", "string")
var sOptPcapNewFilename = getopt.StringLong("file-new", 'n', "", "Filename for the new PCAP file", "string")
var sOptMacAddress = getopt.StringLong("mac", 0, "", "The MAC Address to change in AA:BB:CC:DD:EE:FF format", "string")
var sOptMacAddressNew = getopt.StringLong("mac-new", 0, "", "The replacement MAC Address, required if mac is used", "string")
var sOptIPv4Address = getopt.StringLong("ip4", 0, "", "The IPv4 Address to change", "string")
var sOptIPv4AddressNew = getopt.StringLong("ip4-new", 0, "", "The replacement IPv4 Address, required if ip4 is used", "string")

var iOptNewYear = getopt.IntLong("year", 'y', 0, "Rebase to Year (yyyy)", "int")
var iOptNewMonth = getopt.IntLong("month", 'm', 0, "Rebase to Month (mm)", "int")
var iOptNewDay = getopt.IntLong("day", 'd', 0, "Rebase to Day (dd)", "int")
var sOptTimeShift = getopt.StringLong("time-shift", 0, "", "Rebase Time of Day (+/-00h00m00s) supports multiple values separated by a comma", "string")

var bOptHelp = getopt.BoolLong("help", 0, "Help")
var bOptVer = getopt.BoolLong("version", 0, "Version")

var iDebug = 0
var sVersion = "1.41"

//
//
//
// --------------------------------------------------------------------------------
// Function Main
// --------------------------------------------------------------------------------
func main() {
	getopt.HelpColumn = 26
	getopt.SetParameters("")
	getopt.Parse()
	checkCommandLineOptions()

	// Figure out if there is a change needed for the date of each packet.  We will
	// compute the difference between what is in the first packet and what was passed
	// in via the command line arguments.
	pcapStartTimestamp := header.GetFirstPacketTimestamp(*sOptPcapSrcFilename)
	iDiffYear, iDiffMonth, iDiffDay := header.ComputeNeededPacketDateChange(*iOptNewYear, *iOptNewMonth, *iOptNewDay, pcapStartTimestamp)

	// Allow for multiple time shifts to be passed in at once
	timeShifts := strings.Split(*sOptTimeShift, ",")

	// Parse layer 2 addresses
	userSuppliedMacAddress := layer2.ParseSuppliedLayer2Address(*sOptMacAddress)
	userSuppliedMacAddressNew := layer2.ParseSuppliedLayer2Address(*sOptMacAddressNew)

	// Parse layer 3 IPv4 address
	userSuppliedIPv4Address := layer3.ParseSuppliedLayer3IPv4Address(*sOptIPv4Address)
	userSuppliedIPv4AddressNew := layer3.ParseSuppliedLayer3IPv4Address(*sOptIPv4AddressNew)

	//
	// Get a handle to the PCAP source file so we can loop through each packet and make
	// changes as needed.
	handle, err1 := pcap.OpenOffline(*sOptPcapSrcFilename)
	if err1 != nil {
		fmt.Println(err1)
		os.Exit(0)
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	// Create file handle to write to
	fileHandle, err2 := os.Create(*sOptPcapNewFilename)
	if err2 != nil {
		fmt.Println(err2)
		os.Exit(0)
	}
	writer := pcapgo.NewWriter(fileHandle)
	writer.WriteFileHeader(65535, handle.LinkType())

	fmt.Println("Each '.' represents 1000 packets converted.")

	// -------------------------------------------------------------------------
	// Define counters for status
	// -------------------------------------------------------------------------
	iTotalPacketCounter := 0
	iArpCounter := 0
	i802dot1QCounter := 0
	i802dot1QinQCounter := 0

	// -------------------------------------------------------------------------
	// Loop through every packet and update them as needed writing the changes
	// out to a new file
	// -------------------------------------------------------------------------
	for packet := range packetSource.Packets() {
		if iDebug == 1 {
			fmt.Println("DEBUG: ", "----------------------------------------")
		}

		// ---------------------------------------------------------------------
		// Change timestamps in the PCAP header as needed
		// ---------------------------------------------------------------------
		if iDiffYear != 0 || iDiffMonth != 0 || iDiffDay != 0 {
			header.ChangeTimestampDate(packet, iDiffYear, iDiffMonth, iDiffDay)
		}

		if *sOptTimeShift != "" {
			// Allow for multiple time shifts to be passed at once
			for _, ts := range timeShifts {
				header.ChangeTimestampTimeOfDay(packet, ts)
			}

		}

		// ---------------------------------------------------------------------
		// Change layer 2 MAC addresses as needed
		// ---------------------------------------------------------------------
		if *sOptMacAddress != "" && *sOptMacAddressNew != "" {
			layer2.ReplaceMacAddresses(packet, userSuppliedMacAddress, userSuppliedMacAddressNew)
		}

		i802dot1QOffset := 0
		// ---------------------------------------------------------------------
		// Look for an 802.1Q frames
		// ---------------------------------------------------------------------
		if packet.LinkLayer().LayerContents()[12] == 81 && packet.LinkLayer().LayerContents()[13] == 0 {
			if iDebug == 1 {
				fmt.Println("DEBUG: Found an 802.1Q packet")
			}
			i802dot1QOffset = 4
			i802dot1QCounter++
		}

		// ---------------------------------------------------------------------
		// Look for an 802.1QinQ frame
		// ---------------------------------------------------------------------
		if packet.LinkLayer().LayerContents()[12] == 88 && packet.LinkLayer().LayerContents()[13] == 168 {
			if iDebug == 1 {
				fmt.Println("DEBUG: Found an 802.1QinQ packet")
			}
			i802dot1QOffset = 8
			i802dot1QinQCounter++
		}

		// ---------------------------------------------------------------------
		// Look for an ARP frame.  If it is an ARP packet, we may need update the
		// internal MAC and IP addresses.
		// ---------------------------------------------------------------------
		iEthType1 := 12 + i802dot1QOffset
		iEthType2 := 13 + i802dot1QOffset
		if packet.LinkLayer().LayerContents()[iEthType1] == 8 && packet.LinkLayer().LayerContents()[iEthType2] == 6 {
			if iDebug == 1 {
				fmt.Println("DEBUG: Found an ARP packet")
			}

			// Fix the MAC addresses in the ARP payload if we are fixing MAC addresses at layer 2
			if *sOptMacAddress != "" && *sOptMacAddressNew != "" {
				arp.ReplaceArpPayloadMacAddresses(packet, i802dot1QOffset, userSuppliedMacAddress, userSuppliedMacAddressNew)
			}

			// Fix the IP addresses in the ARP payload if we are changing layer 3 information
			if *sOptIPv4Address != "" && *sOptIPv4AddressNew != "" {
				arp.ReplaceArpPayloadIPv4Addresses(packet, i802dot1QOffset, userSuppliedIPv4Address, userSuppliedIPv4AddressNew)
			}

			iArpCounter++
		} // End ARP Packets

		// ---------------------------------------------------------------------
		// Change Layer 3 information
		// ---------------------------------------------------------------------
		if *sOptIPv4Address != "" && *sOptIPv4AddressNew != "" {
			layer3.ReplaceIPv4Addresses(packet, i802dot1QOffset, userSuppliedIPv4Address, userSuppliedIPv4AddressNew)
		}

		//
		// Write the packet out to the new file
		writer.WritePacket(packet.Metadata().CaptureInfo, packet.Data())

		// Write some output to the screen so users know we are doing something
		iTotalPacketCounter++
		if iTotalPacketCounter%1000 == 0 {
			fmt.Print(".")
			if iTotalPacketCounter%80000 == 0 {
				fmt.Print("\n")
			}
		} // screen feedback

	} // End loop through every packet

	fileHandle.Close()
	fmt.Println("\nTotal number of packets processed:", iTotalPacketCounter)
	fmt.Println("Total number of ARP packets processed:", iArpCounter)
	fmt.Println("Total number of 802.1Q packets processed:", i802dot1QCounter)
	fmt.Println("Total number of 802.1QinQ packets processed:", i802dot1QinQCounter)

} // main()

//
// --------------------------------------------------------------------------------
// checkCommandLineOptions()
// --------------------------------------------------------------------------------
// Verify that all of the command line options meet the required dependencies
func checkCommandLineOptions() {
	if *bOptVer {
		fmt.Println("rewritecap, copyright Bret Jordan, 2015")
		fmt.Println("Version:", sVersion)
		fmt.Println("")
		os.Exit(0)
	}

	if *bOptHelp || (*sOptPcapSrcFilename == "" || *sOptPcapNewFilename == "") {
		fmt.Println("rewritecap, copyright Bret Jordan, 2015")
		fmt.Println("Version:", sVersion)
		fmt.Println("")
		getopt.Usage()
		os.Exit(0)
	}

	if *sOptPcapSrcFilename == *sOptPcapNewFilename {
		fmt.Println("rewritecap, copyright Bret Jordan, 2015")
		fmt.Println("Version:", sVersion)
		fmt.Println("")
		fmt.Println("Filenames are the same.")
		os.Exit(0)

	}

	// Make sure if the user supplies a Layer2 address, that they also supply the other
	if (*sOptMacAddress != "" && *sOptMacAddressNew == "") || (*sOptMacAddressNew != "" && *sOptMacAddress == "") {
		fmt.Println("rewritecap, copyright Bret Jordan, 2015")
		fmt.Println("Version:", sVersion)
		fmt.Println("")
		getopt.Usage()
		os.Exit(0)
	}

	// Make sure if the user supplies a Layer3 address, that they also supply the other
	if (*sOptIPv4Address != "" && *sOptIPv4AddressNew == "") || (*sOptIPv4AddressNew != "" && *sOptIPv4Address == "") {
		fmt.Println("rewritecap, copyright Bret Jordan, 2015")
		fmt.Println("Version:", sVersion)
		fmt.Println("")
		getopt.Usage()
		os.Exit(0)
	}
} //checkCommandLineOptions()
