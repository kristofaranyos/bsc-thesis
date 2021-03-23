#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/random.h>

#define bpf_printk(fmt, ...)                            \
({                                                      \
        char ____fmt[] = fmt;                           \
        bpf_trace_printk(____fmt, sizeof(____fmt),      \
                         ##__VA_ARGS__);                \
})

#define DROP_PKT        0
#define ALLOW_PKT       1

static __u64 t_last = 0;
static __u64 time = 0;
static const __u32 bps = BANDWIDTH;
static const __u64 NSEC_PER_SEC = 1000000000;

SEC("cgroup_skb/egress")
int pkt_tbf(struct __sk_buff *skb)
{
	__u64 now = bpf_ktime_get_ns();
	__u64 t = skb->tstamp;

bpf_printk("now: %u, t: %u\n", now, t);

	if (t < now) {
	    t = now;
	    bpf_printk("now = t");
	}

	__u64 delay = skb->len * NSEC_PER_SEC / bps;
	__u64 t_next = t_last + delay;

	bpf_printk("delay: %u, tnext: %u\n", delay, t_next);

	if (t_next <= t) {
	    t_last = t;
	    bpf_printk("Allowed packet\n");
	    return 1;
	}

	if (t_next - now >= NSEC_PER_SEC * 60) {
    	bpf_printk("Dropped packet\n");
	    return 0;
	}

	t_last = t_next;
	skb->tstamp = t_next;
	bpf_printk("Allowed packet\n");
	return 1;
}

char _license[] SEC("license") = "GPL";