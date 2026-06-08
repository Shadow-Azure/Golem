执行`sh release.sh`重新编译构建，完成下面测试，并将测试结果保存到release_test/${{yyyy-mm-dd-hh-mm-ss}}文件的markdown中
1. 使用cli-box，./golem chat进入交互式CLI，然后发送“你是谁？”，查看返回结果是否符合预期。每一步都截图，并检查截图结果是否符合预期。
2. 使用cli-box，./golem web启动服务端，然后使用chrome dev tool进入对应url，检查返回结果，并输入“你是谁？”，并发送，查看是否符合预期