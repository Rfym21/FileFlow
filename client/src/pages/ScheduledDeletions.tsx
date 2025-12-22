import ScheduledDeletionsManager from "@/components/ScheduledDeletionsManager";

/**
 * 计划删除列表页面
 */
export default function ScheduledDeletions() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">计划删除列表</h1>
        <p className="text-muted-foreground">查看和管理设置了有效期的文件，到期后将自动删除</p>
      </div>

      <ScheduledDeletionsManager />
    </div>
  );
}
