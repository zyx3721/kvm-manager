package kvm

import (
	"fmt"
	"strings"
)

func (p *VirshProvider) validateHostInterfaceAddressUniqueness(request HostInterfaceCreateRequest) error {
	items, err := p.ListHostInterfaces()
	if err != nil {
		return fmt.Errorf("validate interface addresses failed: %w", err)
	}
	if err := validateRequestedAddressUniqueness(request.IPv4Mode, request.IPv4Address, "ipv4", request.Device, items); err != nil {
		return err
	}
	return validateRequestedAddressUniqueness(request.IPv6Mode, request.IPv6Address, "ipv6", request.Device, items)
}

func validateRequestedAddressUniqueness(mode string, address string, family string, allowedSourceDevice string, existing []HostInterface) error {
	if strings.ToLower(strings.TrimSpace(mode)) != "static" {
		return nil
	}
	requestIP, requestNetwork, err := parseInterfaceCIDR(address, family)
	if err != nil {
		return err
	}
	for _, item := range existing {
		if isAllowedBridgeSourceAddress(item, allowedSourceDevice) {
			continue
		}
		existingAddress := item.IPv4
		if family == "ipv6" {
			existingAddress = item.IPv6
		}
		existingIP, existingNetwork, err := parseInterfaceCIDR(existingAddress, family)
		if err != nil {
			continue
		}
		if requestIP.Equal(existingIP) {
			return fmt.Errorf("%s address already exists on interface %s", family, item.Name)
		}
		if requestNetwork.Contains(existingIP) || existingNetwork.Contains(requestIP) {
			return fmt.Errorf("%s subnet already exists on interface %s", family, item.Name)
		}
	}
	return nil
}

func isAllowedBridgeSourceAddress(item HostInterface, allowedSourceDevice string) bool {
	allowedSourceDevice = strings.TrimSpace(allowedSourceDevice)
	if allowedSourceDevice == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(item.Name), allowedSourceDevice)
}
