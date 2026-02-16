import { useEffect, useState } from "react";
import { fluxClient } from "@/lib/flux-client";
import type { TaskEvent } from "@proto/flux/v1/flux_pb";
import { TaskEvent_TaskEventType } from "@proto/flux/v1/flux_pb";

export function useTaskStream(taskId: string | undefined) {
  const [events, setEvents] = useState<TaskEvent[]>([]);
  const [isRunning, setIsRunning] = useState(true);

  useEffect(() => {
    if (!taskId) return;
    const controller = new AbortController();

    (async () => {
      try {
        for await (const resp of fluxClient.streamTaskEvents(
          { taskId },
          { signal: controller.signal }
        )) {
          const event = resp.event;
          if (event) {
            setEvents((prev) => [...prev, event]);
            if (
              event.type === TaskEvent_TaskEventType.TASK_COMPLETE ||
              event.type === TaskEvent_TaskEventType.TASK_ERROR
            ) {
              setIsRunning(false);
            }
          }
        }
      } catch (e) {
        if (!controller.signal.aborted) setIsRunning(false);
      }
    })();

    return () => controller.abort();
  }, [taskId]);

  return { events, isRunning };
}
