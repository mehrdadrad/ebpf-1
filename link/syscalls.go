package link

import (
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/unix"

	"golang.org/x/xerrors"
)

var haveProgAttach = internal.FeatureTest("BPF_PROG_ATTACH", "4.10", func() (bool, error) {
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Type:       ebpf.CGroupSKB,
		AttachType: ebpf.AttachCGroupInetIngress,
		License:    "MIT",
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
	})
	if err != nil {
		return false, nil
	}

	// BPF_PROG_ATTACH was introduced at the same time as CGgroupSKB,
	// so being able to load the program is enough to infer that we
	// have the syscall.
	prog.Close()
	return true, nil
})

var haveProgAttachReplace = internal.FeatureTest("BPF_PROG_ATTACH atomic replacement", "5.5", func() (bool, error) {
	if err := haveProgAttach(); err != nil {
		return false, err
	}

	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Type:       ebpf.CGroupSKB,
		AttachType: ebpf.AttachCGroupInetIngress,
		License:    "MIT",
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
	})
	if err != nil {
		return false, nil
	}
	defer prog.Close()

	// We know that we have BPF_PROG_ATTACH since we can load CGroupSKB programs.
	// If passing BPF_F_REPLACE gives us EINVAL we know that the feature isn't
	// present.
	attr := internal.BPFProgAttachAttr{
		// We rely on this being checked after attachFlags.
		TargetFd:    ^uint32(0),
		AttachBpfFd: uint32(prog.FD()),
		AttachType:  uint32(ebpf.AttachCGroupInetIngress),
		AttachFlags: uint32(flagReplace),
	}

	err = internal.BPFProgAttach(&attr)
	if xerrors.Is(err, unix.EPERM) {
		// We don't have enough permissions, so we never get to the point
		// where flags are checked.
		return false, err
	}
	return !xerrors.Is(err, unix.EINVAL), nil
})

type bpfLinkCreateAttr struct {
	progFd     uint32
	targetFd   uint32
	attachType ebpf.AttachType
	flags      uint32
}

func bpfLinkCreate(attr *bpfLinkCreateAttr) (*internal.FD, error) {
	ptr, err := internal.BPF(internal.BPF_LINK_CREATE, unsafe.Pointer(attr), unsafe.Sizeof(*attr))
	if err != nil {
		return nil, err
	}
	return internal.NewFD(uint32(ptr)), nil
}

type bpfLinkUpdateAttr struct {
	linkFd    uint32
	newProgFd uint32
	flags     uint32
	oldProgFd uint32
}

func bpfLinkUpdate(attr *bpfLinkUpdateAttr) error {
	_, err := internal.BPF(internal.BPF_LINK_UPDATE, unsafe.Pointer(attr), unsafe.Sizeof(*attr))
	return err
}

var haveBPFLink = internal.FeatureTest("bpf_link", "5.7", func() (bool, error) {
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Type:       ebpf.CGroupSKB,
		AttachType: ebpf.AttachCGroupInetIngress,
		License:    "MIT",
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
	})
	if err != nil {
		return false, nil
	}
	defer prog.Close()

	attr := bpfLinkCreateAttr{
		// This is a hopefully invalid file descriptor, which triggers EBADF.
		targetFd:   ^uint32(0),
		progFd:     uint32(prog.FD()),
		attachType: ebpf.AttachCGroupInetIngress,
	}
	_, err = bpfLinkCreate(&attr)
	return !xerrors.Is(err, unix.EINVAL), nil
})