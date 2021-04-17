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

SEC("cgroup_skb/egress")
int pkt_tbf(struct __sk_buff *skb)
{
	__u32 rnd = bpf_get_prandom_u32() % 100;
	bpf_printk("Random number: %d\n", rnd);

    if (DISTRIBUTION == 0) {
        if(rnd <= PERCENTAGE) {
            bpf_printk("Dropped packet");
            return DROP_PKT;
        }

        bpf_printk("Allowed packet");
        return ALLOW_PKT;
    }

    if (DISTRIBUTION == 1) {
    int a = bpf_log2l(2);
        return ALLOW_PKT;
    }
}

char _license[] SEC("license") = "GPL";
