package libcore

import (
	"net"
	"sync/atomic"
)

type AppStats struct {
	Uid          int32
	TcpConn      int32
	UdpConn      int32
	TcpConnTotal int32
	UdpConnTotal int32

	Uplink        int64
	Downlink      int64
	UplinkTotal   int64
	DownlinkTotal int64

	DeactivateAt int32
}

type appStats struct {
	tcpConn      int32
	udpConn      int32
	tcpConnTotal uint32
	udpConnTotal uint32

	uplink        uint64
	downlink      uint64
	uplinkTotal   uint64
	downlinkTotal uint64

	deactivateAt int64
}

type TrafficListener interface {
	UpdateStats(t *AppStats)
}

func (t *Tun2ray) GetTrafficStatsEnabled() bool {
	return t.trafficStats
}

func (t *Tun2ray) ResetAppTraffics() {
	if !t.trafficStats {
		return
	}

	t.access.RLock()
	var toDel []uint16
	for uid, stat := range t.appStats {
		atomic.StoreUint64(&stat.uplink, 0)
		atomic.StoreUint64(&stat.downlink, 0)
		atomic.StoreUint64(&stat.uplinkTotal, 0)
		atomic.StoreUint64(&stat.downlinkTotal, 0)
		if stat.tcpConn+stat.udpConn == 0 {
			toDel = append(toDel, uid)
		}
	}
	t.access.RUnlock()
	if len(toDel) > 0 {
		t.access.Lock()
		for _, uid := range toDel {
			delete(t.appStats, uid)
		}
		t.access.Unlock()
	}

}

func (t *Tun2ray) ReadAppTraffics(listener TrafficListener) error {
	if !t.trafficStats {
		return nil
	}

	var stats []*AppStats
	t.access.RLock()
	for uid, stat := range t.appStats {
		export := &AppStats{
			Uid:          int32(uid),
			TcpConn:      stat.tcpConn,
			UdpConn:      stat.udpConn,
			TcpConnTotal: int32(stat.tcpConnTotal),
			UdpConnTotal: int32(stat.udpConnTotal),
			DeactivateAt: int32(stat.deactivateAt),
		}

		uplink := atomic.SwapUint64(&stat.uplink, 0)
		uplinkTotal := atomic.AddUint64(&stat.uplinkTotal, uplink)
		export.Uplink = int64(uplink)
		export.UplinkTotal = int64(uplinkTotal)

		downlink := atomic.SwapUint64(&stat.downlink, 0)
		downlinkTotal := atomic.AddUint64(&stat.downlinkTotal, downlink)
		export.Downlink = int64(downlink)
		export.DownlinkTotal = int64(downlinkTotal)

		stats = append(stats, export)
	}
	t.access.RUnlock()

	for _, stat := range stats {
		listener.UpdateStats(stat)
	}

	return nil
}

type statsConn struct {
	net.Conn
	uplink   *uint64
	downlink *uint64
}

func (c *statsConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	defer atomic.AddUint64(c.uplink, uint64(n))
	return
}

func (c *statsConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	defer atomic.AddUint64(c.downlink, uint64(n))
	return
}

type statsPacketConn struct {
	packetConn
	uplink   *uint64
	downlink *uint64
}

func (c statsPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.packetConn.ReadFrom(p)
	if err == nil {
		atomic.AddUint64(c.downlink, uint64(n))
	}
	return
}

func (c statsPacketConn) readFrom() (p []byte, addr net.Addr, err error) {
	p, addr, err = c.packetConn.readFrom()
	if err == nil {
		atomic.AddUint64(c.downlink, uint64(len(p)))
	}
	return
}

func (c statsPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	n, err = c.packetConn.WriteTo(p, addr)
	if err == nil {
		atomic.AddUint64(c.uplink, uint64(n))
	}
	return
}
