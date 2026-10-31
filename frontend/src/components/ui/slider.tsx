import * as React from "react";
import { Slider as SliderPrimitive } from "radix-ui";

import { cn } from "@/utils/cn";

function Slider({
  className,
  defaultValue,
  value,
  min = 0,
  max = 100,
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root>) {
  const _value = Array.isArray(value)
    ? value
    : Array.isArray(defaultValue)
      ? defaultValue
      : [min, max];

  return (
    <SliderPrimitive.Root
      data-slot="slider"
      value={value}
      defaultValue={defaultValue}
      min={min}
      max={max}
      className={cn(
        "relative flex w-full touch-none select-none data-[disabled]:opacity-50 data-[orientation=horizontal]:h-2 data-[orientation=vertical]:h-full data-[orientation=vertical]:min-h-[20px] data-[orientation=horizontal]:min-w-[20px] data-[orientation=vertical]:w-2 data-[orientation=horizontal]:flex-row data-[orientation=vertical]:flex-col",
        className,
      )}
      {...props}
    >
      <SliderPrimitive.Track
        data-slot="slider-track"
        className="bg-muted relative data-[orientation=horizontal]:h-full data-[orientation=horizontal]:w-full data-[orientation=vertical]:h-full data-[orientation=vertical]:w-0.5"
      >
        <SliderPrimitive.Range
          data-slot="slider-range"
          className="bg-primary absolute data-[orientation=horizontal]:h-full data-[orientation=vertical]:w-full"
        />
      </SliderPrimitive.Track>
      {Array.from({ length: _value.length }, (_, i) => (
        <SliderPrimitive.Thumb
          data-slot="slider-thumb"
          key={i}
          className="border-primary/60 bg-background ring-ring/50 block size-3.5 shrink-0 rounded-full border shadow-sm transition-[color,box-shadow] [--shadow-color:oklch(0.93_0_0)] hover:ring-2 focus-visible:ring-2 focus-visible:outline-hidden disabled:pointer-events-none disabled:opacity-50 data-[disabled]:pointer-events-none"
        />
      ))}
    </SliderPrimitive.Root>
  );
}

export { Slider };
