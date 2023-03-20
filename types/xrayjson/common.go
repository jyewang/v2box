package xrayjson

import (
	"net/url"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	v2ray_net "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/infra/conf"
)

func parseServerAddress(servers []*protocol.ServerEndpoint) (M.Socksaddr, []*protocol.User) {
	if len(servers) == 0 {
		return M.Socksaddr{}, nil
	}
	return M.ParseSocksaddrHostPort(servers[0].Address.AsAddress().String(), uint16(servers[0].Port)), servers[0].User
}

func parsePort(portList *conf.PortList) uint16 {
	if portList == nil {
		return 0
	}
	netPortList := portList.Build()
	if len(netPortList.Range) == 0 {
		return 0
	}
	return uint16(netPortList.Range[0].From)
}

func parseNetworkList(networks *conf.NetworkList) string {
	if networks == nil {
		return ""
	}
	networkList := networks.Build()
	networkList = common.Filter(networkList, func(it v2ray_net.Network) bool {
		return it == v2ray_net.Network_TCP || it == v2ray_net.Network_UDP
	})
	if len(networkList) == 1 {
		switch networkList[0] {
		case v2ray_net.Network_TCP:
			return N.NetworkTCP
		case v2ray_net.Network_UDP:
			return N.NetworkUDP
		}
	}
	return ""
}

func parseNetworks(networks []v2ray_net.Network) string {
	networks = common.Filter(networks, func(it v2ray_net.Network) bool {
		return it == v2ray_net.Network_TCP || it == v2ray_net.Network_UDP
	})
	if len(networks) == 1 {
		switch networks[0] {
		case v2ray_net.Network_TCP:
			return N.NetworkTCP
		case v2ray_net.Network_UDP:
			return N.NetworkUDP
		}
	}
	return ""
}

func parseTransport(streamSettings *conf.StreamConfig) (option.V2RayTransportOptions, error) {
	if streamSettings.Network == nil {
		return option.V2RayTransportOptions{}, nil
	}
	var transportOptions option.V2RayTransportOptions
	networkName, err := streamSettings.Network.Build()
	if err != nil {
		return option.V2RayTransportOptions{}, err
	}
	switch networkName {
	case "tcp":
		if tcpSettings := streamSettings.TCPSettings; tcpSettings != nil {
			if tcpSettings.HeaderConfig != nil {
				return option.V2RayTransportOptions{}, E.New("unsupported v2ray TCP transport with header")
			}
		}
	case "http":
		transportOptions.Type = C.V2RayTransportTypeHTTP
		if httpSettings := streamSettings.HTTPSettings; httpSettings != nil {
			if httpSettings.Host != nil {
				transportOptions.HTTPOptions.Host = []string(*httpSettings.Host)
			}
			transportOptions.HTTPOptions.Path = httpSettings.Path
			transportOptions.HTTPOptions.Method = httpSettings.Method
			if len(httpSettings.Headers) > 0 {
				transportOptions.HTTPOptions.Headers = make(map[string]string)
				for key, value := range httpSettings.Headers {
					if value == nil || value.Len() == 0 {
						continue
					}
					transportOptions.HTTPOptions.Headers[key] = (*value)[0]
				}
			}
		}
	case "ws":
		transportOptions.Type = C.V2RayTransportTypeWebsocket
		if wsSettings := streamSettings.WSSettings; wsSettings != nil {
			if wsSettings.Headers != nil {
				transportOptions.WebsocketOptions.Headers = make(map[string]string)
				for key, value := range wsSettings.Headers {
					transportOptions.WebsocketOptions.Headers[key] = value
				}
			}
			if wsSettings.Path != "" {
				if !strings.HasPrefix(wsSettings.Path, "/") {
					wsSettings.Path = "/" + wsSettings.Path
				}
				pathURL, err := url.Parse(wsSettings.Path)
				if err != nil {
					return option.V2RayTransportOptions{}, err
				}
				pathQuery := pathURL.Query()
				maxEarlyDataString := pathQuery.Get("ed")
				if maxEarlyDataString != "" {
					maxEarlyDate, err := strconv.ParseUint(maxEarlyDataString, 10, 32)
					if err == nil {
						transportOptions.WebsocketOptions.MaxEarlyData = uint32(maxEarlyDate)
						transportOptions.WebsocketOptions.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
						pathQuery.Del("ed")
						pathURL.RawQuery = pathQuery.Encode()
					}
				}
				transportOptions.WebsocketOptions.Path = pathURL.String()
			}
		}
	case "grpc", "gun":
		transportOptions.Type = C.V2RayTransportTypeGRPC
		transportOptions.Type = C.V2RayTransportTypeGRPC
		if grpcSettings := streamSettings.GRPCConfig; grpcSettings != nil {
			transportOptions.GRPCOptions.ServiceName = grpcSettings.ServiceName
		} else if grpcSettings := streamSettings.GUNConfig; grpcSettings != nil {
			transportOptions.GRPCOptions.ServiceName = grpcSettings.ServiceName
		}
	case "quic":
		transportOptions.Type = C.V2RayTransportTypeQUIC
	default:
		return option.V2RayTransportOptions{}, E.New("unsupported v2ray transport type: ", networkName)
	}
	return transportOptions, nil
}
