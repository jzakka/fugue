"use client";

import { useEffect } from "react";
import { recordInteraction } from "@/lib/api";

export default function PinDetailTracker({ pinId }: { pinId: string }) {
  useEffect(() => {
    recordInteraction(pinId, "view");
  }, [pinId]);

  return null;
}
