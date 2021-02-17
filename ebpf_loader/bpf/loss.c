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

static __s64 tokens = 12500;
static __u64 time = 0;
static const __u32 bps = 12500; //speed in bytes/sec (10Mbps now)
static const __u32 extra_tokens = bps >> 9; //few extra tokens for smoother operation

SEC("cgroup_skb/egress")
int pkt_tbf(struct __sk_buff *skb)
{
	unsigned i = bpf_get_prandom_u32() % 100;
	bpf_printk("Random number: %d\n", i);

	if(i < 50.)
	{
		bpf_printk("Drop pkt: %d\n", tokens);
		tokens = 0;
		return DROP_PKT;
	}

	bpf_printk("Allow, tokens: %lld\n", tokens);
	return ALLOW_PKT;
}

char _license[] SEC("license") = "GPL";
