# lab07

## networking

按照hints做就可以了

```c
int
e1000_transmit(struct mbuf *m)
{
  uint32 tdt;

  acquire(&e1000_lock);
  tdt = regs[E1000_TDT];
  if ((tx_ring[tdt].status & E1000_TXD_STAT_DD) == 0){
    release(&e1000_lock);
    return -1;
  }
  if (tx_mbufs[tdt])
    mbuffree(tx_mbufs[tdt]);
  tx_mbufs[tdt] = m;
  tx_ring[tdt].addr = (uint64)m->head; 
  tx_ring[tdt].cmd = E1000_TXD_CMD_EOP | E1000_TXD_CMD_RS;
  tx_ring[tdt].length = m->len;
  regs[E1000_TDT] = (tdt + 1) % TX_RING_SIZE;
  release(&e1000_lock);
  return 0;
}

static void
e1000_recv(void)
{
  uint32 rdt;
  
  rdt = (regs[E1000_RDT] + 1) % RX_RING_SIZE;
  for (; rdt != regs[E1000_RDH]; rdt = (rdt + 1) % RX_RING_SIZE) {
    if ((rx_ring[rdt].status & E1000_RXD_STAT_DD) == 0)
      break;
    rx_mbufs[rdt]->len = rx_ring[rdt].length;
    net_rx(rx_mbufs[rdt]);
    rx_mbufs[rdt] = mbufalloc(0);
    rx_ring[rdt].addr = (uint64)rx_mbufs[rdt]->head;
    // update RDT
    regs[E1000_RDT] = rdt;
  }
}
```