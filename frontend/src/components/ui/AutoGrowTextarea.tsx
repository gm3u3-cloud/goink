import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
} from "react";

interface AutoGrowTextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** Minimum height (px). If omitted, no lower bound is enforced. */
  minHeight?: number;
  /** Maximum height (px); a scrollbar appears once exceeded. Default 160. */
  maxHeight?: number;
}

// AutoGrowTextarea is a textarea that auto-adjusts its height based on
// content: it initially shows at minHeight (or rows/CSS), grows as input
// increases, and stops growing once maxHeight is reached (then scrolls).
// Reuses ChatInput's scrollHeight pattern, extracted into a generic
// component for reuse in approval dialogs and elsewhere.
const AutoGrowTextarea = forwardRef<HTMLTextAreaElement, AutoGrowTextareaProps>(
  (
    { minHeight, maxHeight = 160, value, onChange, className, ...rest },
    ref,
  ) => {
    const innerRef = useRef<HTMLTextAreaElement>(null);
    useImperativeHandle(ref, () => innerRef.current as HTMLTextAreaElement);

    const adjust = useCallback(() => {
      const el = innerRef.current;
      if (!el) return;
      el.style.height = "auto";
      let h = el.scrollHeight;
      if (maxHeight && h > maxHeight) h = maxHeight;
      if (minHeight && h < minHeight) h = minHeight;
      el.style.height = h + "px";
    }, [minHeight, maxHeight]);

    // Adjust on controlled value change (programmatic set also triggers it)
    useEffect(() => {
      adjust();
    }, [value, adjust]);

    return (
      <textarea
        ref={innerRef}
        value={value}
        onChange={(e) => {
          onChange?.(e);
          adjust();
        }}
        className={className}
        {...rest}
      />
    );
  },
);

AutoGrowTextarea.displayName = "AutoGrowTextarea";

export default AutoGrowTextarea;
