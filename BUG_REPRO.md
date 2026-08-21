# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

两个地面调度员几乎同时把货物仓位从 A 航站楼转到 B、又从 B 转回 A，双方放行确认都已完成，但两个请求随后一起挂住，连这两个航站楼的其他仓位操作也不再返回；单向转移一直正常。请先不要修改代码，查清对向转移为何互相等待，说明两份日历、外部放行和锁持有顺序之间的完整时序及影响边界。

## 含 Bug 版本

- 仓库：VanceMichael/go-label-23
- 仓库地址：https://github.com/VanceMichael/go-label-23.git
- parent SHA：127db924e4512c018c763617408d1ccfae6a2455

## 复现步骤

```bash
git clone -- https://github.com/VanceMichael/go-label-23.git bug-repro
cd bug-repro
git checkout --detach 127db924e4512c018c763617408d1ccfae6a2455
go test ./internal/warehouse -run ^TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther$ -count=1
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/warehouse -run ^TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther$ -count=1
--- FAIL: TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther (0.21s)
    --- FAIL: TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther/opposite_transfers (0.20s)
        transfer_test.go:73: opposite terminal transfers blocked each other
FAIL
FAIL	github.com/VanceMichael/go-base-airbridge/internal/warehouse	0.233s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/warehouse -run ^TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther$ -count=1
--- FAIL: TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther (0.20s)
    --- FAIL: TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther/opposite_transfers (0.20s)
        transfer_test.go:73: opposite terminal transfers blocked each other
FAIL
FAIL	github.com/VanceMichael/go-base-airbridge/internal/warehouse	0.202s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

根因结论必须定位对向仓位转移中两份日历形成循环等待的具体调用阶段，完整说明外部放行发生时源锁仍被持有、双方随后各自等待目标锁，以及为何单向操作不受影响；定向命令 go test -race ./internal/warehouse -run '^TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther$' -count=1 应稳定重现阻塞，相关调查证据应与固定 main SHA 一致，且目标仓库代码、测试和配置零改动。
