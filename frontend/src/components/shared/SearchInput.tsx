import { forwardRef } from "react";
import { Search, X, Loader2 } from "lucide-react";

interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  placeholder?: string;
  loading?: boolean;
  /** Passed to the inner relative div (e.g. "flex-1"); the outer
   * padding/border is controlled by the caller. */
  className?: string;
}

// SearchInput: unified search-box styling (shared by global search and
// in-domain search).
// Uses SearchPanel's styling standard: icon w-3.5 + input h-7 + optional
// clear button. The caller is responsible for the outer wrapper
// (padding/border); this component only handles the input area.
const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  ({ value, onChange, onKeyDown, placeholder, loading, className }, ref) => {
    return (
      <div className={`relative ${className ?? ""}`}>
        <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
        <input
          ref={ref}
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          className="w-full h-7 rounded-md border bg-background pl-7 pr-7 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
        {(value || loading) && (
          <button
            onMouseDown={(e) => {
              e.preventDefault(); // Prevent the button from taking focus, keeping the input focused
              onChange("");
            }}
            className="absolute right-1.5 top-1/2 -translate-y-1/2 w-4 h-4 flex items-center justify-center text-muted-foreground hover:text-foreground"
          >
            {loading ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : (
              <X className="w-3 h-3" />
            )}
          </button>
        )}
      </div>
    );
  },
);

SearchInput.displayName = "SearchInput";
export default SearchInput;
