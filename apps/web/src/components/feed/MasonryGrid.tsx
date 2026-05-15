"use client";

import Masonry from "react-masonry-css";
import type { ReactNode } from "react";

const BREAKPOINT_COLUMNS = {
  default: 4,
  1199: 3,
  799: 2,
  499: 1,
};

export default function MasonryGrid({ children }: { children: ReactNode }) {
  return (
    <Masonry
      breakpointCols={BREAKPOINT_COLUMNS}
      className="masonry-grid"
      columnClassName="masonry-grid_column"
    >
      {children}
    </Masonry>
  );
}
