import * as React from "react"

import { cn } from "../../lib/utils"

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> { }

const Input = React.forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => {
    return (
        <input
            type={type}
            className={cn(
                // Layout
                "flex w-full rounded px-3 py-2 text-sm",
                // Colors - EXPLICIT DARK (VSCode variable unreliable)
                "bg-[#3c3c3c] text-[#cccccc]",
                // NO BORDER
                // Placeholder
                "placeholder:text-vscode-input-foreground/40",
                // Focus state
                "focus:outline-none focus:border-vscode-focusBorder",
                // Disabled state
                "disabled:cursor-not-allowed disabled:opacity-50",
                // Transition
                "transition-colors",
                className
            )}
            ref={ref}
            {...props}
        />
    )
})
Input.displayName = "Input"

export { Input }
