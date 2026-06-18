"use client"

import * as React from "react"
import { cva } from "class-variance-authority"
import { AlertCircleIcon, CheckIcon, PlusIcon, XIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ButtonGroup, ButtonGroupText } from "@/components/ui/button-group"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupText,
} from "@/components/ui/input-group"
import { Kbd } from "@/components/ui/kbd"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

export interface FilterI18nConfig {
  addFilter: string
  searchFields: string
  noFieldsFound: string
  noResultsFound: string
  select: string
  true: string
  false: string
  min: string
  max: string
  to: string
  typeAndPressEnter: string
  selected: string
  selectedCount: string
  percent: string
  defaultCurrency: string
  defaultColor: string
  addFilterTitle: string
  operators: {
    is: string
    isNot: string
    isAnyOf: string
    isNotAnyOf: string
    includesAll: string
    excludesAll: string
    before: string
    after: string
    between: string
    notBetween: string
    contains: string
    notContains: string
    startsWith: string
    endsWith: string
    isExactly: string
    equals: string
    notEquals: string
    greaterThan: string
    lessThan: string
    overlaps: string
    includes: string
    excludes: string
    includesAllOf: string
    includesAnyOf: string
    empty: string
    notEmpty: string
  }
  placeholders: {
    enterField: (fieldType: string) => string
    selectField: string
    searchField: (fieldName: string) => string
    enterKey: string
    enterValue: string
  }
  helpers: {
    formatOperator: (operator: string) => string
  }
  validation: {
    invalidEmail: string
    invalidUrl: string
    invalidTel: string
    invalid: string
  }
}

export const DEFAULT_I18N: FilterI18nConfig = {
  addFilter: "Filter",
  searchFields: "Filter...",
  noFieldsFound: "No filters found.",
  noResultsFound: "No results found.",
  select: "Select...",
  true: "True",
  false: "False",
  min: "Min",
  max: "Max",
  to: "to",
  typeAndPressEnter: "Type and press Enter to add tag",
  selected: "selected",
  selectedCount: "selected",
  percent: "%",
  defaultCurrency: "$",
  defaultColor: "#000000",
  addFilterTitle: "Add filter",
  operators: {
    is: "is",
    isNot: "is not",
    isAnyOf: "is any of",
    isNotAnyOf: "is not any of",
    includesAll: "includes all",
    excludesAll: "excludes all",
    before: "before",
    after: "after",
    between: "between",
    notBetween: "not between",
    contains: "contains",
    notContains: "does not contain",
    startsWith: "starts with",
    endsWith: "ends with",
    isExactly: "is exactly",
    equals: "equals",
    notEquals: "not equals",
    greaterThan: "greater than",
    lessThan: "less than",
    overlaps: "overlaps",
    includes: "includes",
    excludes: "excludes",
    includesAllOf: "includes all of",
    includesAnyOf: "includes any of",
    empty: "is empty",
    notEmpty: "is not empty",
  },
  placeholders: {
    enterField: (fieldType: string) => `Enter ${fieldType}...`,
    selectField: "Select...",
    searchField: (fieldName: string) => `Search ${fieldName.toLowerCase()}...`,
    enterKey: "Enter key...",
    enterValue: "Enter value...",
  },
  helpers: {
    formatOperator: (operator: string) => operator.replace(/_/g, " "),
  },
  validation: {
    invalidEmail: "Invalid email format",
    invalidUrl: "Invalid URL format",
    invalidTel: "Invalid phone format",
    invalid: "Invalid input format",
  },
}

interface FilterContextValue {
  variant: "solid" | "default"
  size: "sm" | "default" | "lg"
  radius: "default" | "full"
  i18n: FilterI18nConfig
  className?: string
  showSearchInput?: boolean
  trigger?: React.ReactNode
  allowMultiple?: boolean
}

const FilterContext = React.createContext<FilterContextValue>({
  variant: "default",
  size: "default",
  radius: "default",
  i18n: DEFAULT_I18N,
  className: undefined,
  showSearchInput: true,
  trigger: undefined,
  allowMultiple: true,
})

function useFilterContext() {
  return React.useContext(FilterContext)
}

const filtersContainerVariants = cva("flex flex-wrap items-center", {
  variants: {
    variant: {
      solid: "gap-2",
      default: "",
    },
    size: {
      sm: "gap-1.5",
      default: "gap-2.5",
      lg: "gap-3.5",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "default",
  },
})

const filterControlHeight = {
  sm: "h-6!",
  default: "h-7!",
  lg: "h-8!",
} as const

export interface FilterOption<T = unknown> {
  value: T
  label: string
  icon?: React.ReactNode
  metadata?: Record<string, unknown>
  className?: string
}

export interface FilterOperator {
  value: string
  label: string
  supportsMultiple?: boolean
}

export interface CustomRendererProps<T = unknown> {
  field: FilterFieldConfig<T>
  values: T[]
  onChange: (values: T[]) => void
  operator: string
}

export interface FilterFieldGroup<T = unknown> {
  group?: string
  fields: FilterFieldConfig<T>[]
}

export type FilterFieldsConfig<T = unknown> =
  | FilterFieldConfig<T>[]
  | FilterFieldGroup<T>[]

export interface FilterFieldConfig<T = unknown> {
  key?: string
  label?: string
  icon?: React.ReactNode
  type?: "select" | "multiselect" | "text" | "custom" | "separator"
  group?: string
  fields?: FilterFieldConfig<T>[]
  options?: FilterOption<T>[]
  operators?: FilterOperator[]
  customRenderer?: (props: CustomRendererProps<T>) => React.ReactNode
  customValueRenderer?: (
    values: T[],
    options: FilterOption<T>[]
  ) => React.ReactNode
  placeholder?: string
  searchable?: boolean
  maxSelections?: number
  min?: number
  max?: number
  step?: number
  prefix?: string | React.ReactNode
  suffix?: string | React.ReactNode
  pattern?: string
  validation?: (
    value: unknown
  ) => boolean | { valid: boolean; message?: string }
  allowCustomValues?: boolean
  className?: string
  menuPopupClassName?: string
  groupLabel?: string
  onLabel?: string
  offLabel?: string
  onInputChange?: (event: React.ChangeEvent<HTMLInputElement>) => void
  defaultOperator?: string
  value?: T[]
  onValueChange?: (values: T[]) => void
}

export interface Filter<T = unknown> {
  id: string
  field: string
  operator: string
  values: T[]
}

export interface FilterGroup<T = unknown> {
  id: string
  label?: string
  filters: Filter<T>[]
  fields: FilterFieldConfig<T>[]
}

function isFieldGroup<T = unknown>(
  item: FilterFieldConfig<T> | FilterFieldGroup<T>
): item is FilterFieldGroup<T> {
  return "fields" in item && Array.isArray(item.fields)
}

function isGroupLevelField<T = unknown>(field: FilterFieldConfig<T>): boolean {
  return Boolean(field.group && field.fields)
}

function flattenFields<T = unknown>(
  fields: FilterFieldsConfig<T>
): FilterFieldConfig<T>[] {
  return fields.reduce<FilterFieldConfig<T>[]>((acc, item) => {
    if (isFieldGroup(item)) {
      return [...acc, ...item.fields]
    }
    if (isGroupLevelField(item)) {
      return [...acc, ...(item.fields ?? [])]
    }
    return [...acc, item]
  }, [])
}

function getFieldsMap<T = unknown>(
  fields: FilterFieldsConfig<T>
): Record<string, FilterFieldConfig<T>> {
  return flattenFields(fields).reduce(
    (acc, field) => {
      if (field.key) {
        acc[field.key] = field
      }
      return acc
    },
    {} as Record<string, FilterFieldConfig<T>>
  )
}

function createOperatorsFromI18n(
  i18n: FilterI18nConfig
): Record<string, FilterOperator[]> {
  return {
    select: [
      { value: "is", label: i18n.operators.is },
      { value: "is_not", label: i18n.operators.isNot },
      { value: "empty", label: i18n.operators.empty },
      { value: "not_empty", label: i18n.operators.notEmpty },
    ],
    multiselect: [
      { value: "is_any_of", label: i18n.operators.isAnyOf },
      { value: "is_not_any_of", label: i18n.operators.isNotAnyOf },
      { value: "includes_all", label: i18n.operators.includesAll },
      { value: "excludes_all", label: i18n.operators.excludesAll },
      { value: "empty", label: i18n.operators.empty },
      { value: "not_empty", label: i18n.operators.notEmpty },
    ],
    text: [
      { value: "contains", label: i18n.operators.contains },
      { value: "not_contains", label: i18n.operators.notContains },
      { value: "starts_with", label: i18n.operators.startsWith },
      { value: "ends_with", label: i18n.operators.endsWith },
      { value: "is", label: i18n.operators.isExactly },
      { value: "empty", label: i18n.operators.empty },
      { value: "not_empty", label: i18n.operators.notEmpty },
    ],
    custom: [
      { value: "is", label: i18n.operators.is },
      { value: "after", label: i18n.operators.after },
      { value: "between", label: i18n.operators.between },
      { value: "empty", label: i18n.operators.empty },
      { value: "not_empty", label: i18n.operators.notEmpty },
    ],
  }
}

export const DEFAULT_OPERATORS: Record<string, FilterOperator[]> =
  createOperatorsFromI18n(DEFAULT_I18N)

function getOperatorsForField<T = unknown>(
  field: FilterFieldConfig<T>,
  values: T[],
  i18n: FilterI18nConfig
): FilterOperator[] {
  if (field.operators) return field.operators

  const operators = createOperatorsFromI18n(i18n)
  let fieldType = field.type ?? "select"

  if (fieldType === "select" && values.length > 1) {
    fieldType = "multiselect"
  }

  if (fieldType === "multiselect") {
    return operators.multiselect
  }

  return operators[fieldType] ?? operators.select
}

function FilterInput<T = unknown>({
  field,
  onBlur,
  onKeyDown,
  className,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & {
  className?: string
  field?: FilterFieldConfig<T>
}) {
  const context = useFilterContext()
  const [isValid, setIsValid] = React.useState(true)
  const [validationMessage, setValidationMessage] = React.useState("")
  const inputRef = React.useRef<HTMLInputElement>(null)

  React.useEffect(() => {
    if (!props.autoFocus) return undefined

    const timer = window.setTimeout(() => {
      inputRef.current?.focus()
    }, 300)

    return () => window.clearTimeout(timer)
  }, [props.autoFocus])

  function validateInput(value: string, pattern?: string): boolean {
    if (!pattern || !value) return true
    return new RegExp(pattern).test(value)
  }

  function handleBlur(event: React.FocusEvent<HTMLInputElement>) {
    const value = event.target.value
    const pattern = field?.pattern ?? props.pattern

    if (value && (pattern || field?.validation)) {
      let valid = true
      let customMessage = ""

      if (field?.validation) {
        const result = field.validation(value)
        if (typeof result === "boolean") {
          valid = result
        } else {
          valid = result.valid
          customMessage = result.message ?? ""
        }
      } else if (pattern) {
        valid = validateInput(value, pattern)
      }

      setIsValid(valid)
      setValidationMessage(
        valid ? "" : customMessage || context.i18n.validation.invalid
      )
    } else {
      setIsValid(true)
      setValidationMessage("")
    }

    onBlur?.(event)
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (
      !isValid &&
      ![
        "Tab",
        "Escape",
        "Enter",
        "ArrowUp",
        "ArrowDown",
        "ArrowLeft",
        "ArrowRight",
      ].includes(event.key)
    ) {
      setIsValid(true)
      setValidationMessage("")
    }

    onKeyDown?.(event)
  }

  return (
    <InputGroup
      className={cn(
        "w-36",
        "has-[[data-slot=input-group-control]:focus-visible]:border-input",
        "has-[[data-slot=input-group-control]:focus-visible]:ring-0",
        "has-[[data-slot=input-group-control]:focus-visible]:ring-transparent",
        filterControlHeight[context.size],
        className
      )}
    >
      {field?.prefix ? (
        <InputGroupAddon>
          <InputGroupText>{field.prefix}</InputGroupText>
        </InputGroupAddon>
      ) : null}
      <InputGroupInput
        ref={inputRef}
        aria-invalid={!isValid}
        aria-describedby={
          !isValid && validationMessage
            ? `${field?.key ?? "input"}-error`
            : undefined
        }
        onBlur={handleBlur}
        onKeyDown={handleKeyDown}
        className={cn(
          "focus-visible:border-transparent focus-visible:ring-0 focus-visible:ring-offset-0",
          filterControlHeight[context.size],
          context.size === "sm" && "text-xs"
        )}
        {...props}
      />
      {!isValid && validationMessage ? (
        <InputGroupAddon align="inline-end">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <InputGroupButton size="icon-xs">
                  <AlertCircleIcon className="size-3.5 text-destructive" />
                </InputGroupButton>
              </TooltipTrigger>
              <TooltipContent>
                <p className="text-sm">{validationMessage}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </InputGroupAddon>
      ) : null}
      {field?.suffix ? (
        <InputGroupAddon align="inline-end">
          <InputGroupText>{field.suffix}</InputGroupText>
        </InputGroupAddon>
      ) : null}
    </InputGroup>
  )
}

function FilterRemoveButton({
  icon = <XIcon />,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  icon?: React.ReactNode
}) {
  const context = useFilterContext()

  return (
    <Button
      variant="outline"
      size={
        context.size === "sm"
          ? "icon-sm"
          : context.size === "lg"
            ? "icon-lg"
            : "icon"
      }
      {...props}
    >
      {icon}
    </Button>
  )
}

function FilterOperatorDropdown<T = unknown>({
  field,
  operator,
  values,
  onChange,
}: {
  field: FilterFieldConfig<T>
  operator: string
  values: T[]
  onChange: (operator: string) => void
}) {
  const context = useFilterContext()
  const operators = getOperatorsForField(field, values, context.i18n)
  const operatorLabel =
    operators.find((item) => item.value === operator)?.label ??
    context.i18n.helpers.formatOperator(operator)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size={context.size}
          className="text-muted-foreground hover:text-foreground"
        >
          {operatorLabel}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="z-[200] w-fit min-w-fit">
        {operators.map((item) => (
          <DropdownMenuItem
            key={item.value}
            onClick={() => onChange(item.value)}
            className="flex items-center justify-between data-highlighted:bg-accent data-highlighted:text-accent-foreground"
          >
            <span>{item.label}</span>
            <CheckIcon
              className={cn(
                "ms-auto size-4 text-primary",
                item.value === operator ? "opacity-100" : "opacity-0"
              )}
            />
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function SelectOptionsPopover<T = unknown>({
  field,
  values,
  onChange,
  onClose,
  inline = false,
}: {
  field: FilterFieldConfig<T>
  values: T[]
  onChange: (values: T[]) => void
  onClose?: () => void
  inline?: boolean
}) {
  const [open, setOpen] = React.useState(false)
  const [searchInput, setSearchInput] = React.useState("")
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1)
  const [kbNav, setKbNav] = React.useState(false)
  const context = useFilterContext()
  const baseId = React.useId()
  const isMultiSelect = field.type === "multiselect" || values.length > 1
  const effectiveValues = field.value !== undefined ? field.value : values
  const selectedOptions =
    field.options?.filter((item) => effectiveValues.includes(item.value)) ?? []
  const unselectedOptions =
    field.options?.filter((item) => !effectiveValues.includes(item.value)) ?? []
  const filteredSelectedOptions = selectedOptions
  const filteredUnselectedOptions = unselectedOptions.filter((item) =>
    item.label.toLowerCase().includes(searchInput.toLowerCase())
  )
  const allFilteredOptions = React.useMemo(
    () => [...filteredSelectedOptions, ...filteredUnselectedOptions],
    [filteredSelectedOptions, filteredUnselectedOptions]
  )

  // Reset the highlight when the search or open state changes — adjusted during
  // render (the React-sanctioned alternative to a setState-in-effect).
  const highlightResetKey = `${searchInput}:${open}`
  const [prevHighlightResetKey, setPrevHighlightResetKey] =
    React.useState(highlightResetKey)
  if (prevHighlightResetKey !== highlightResetKey) {
    setPrevHighlightResetKey(highlightResetKey)
    setHighlightedIndex(-1)
  }

  React.useEffect(() => {
    if (highlightedIndex >= 0 && open && kbNav) {
      document
        .getElementById(`${baseId}-item-${highlightedIndex}`)
        ?.scrollIntoView({ block: "nearest" })
    }
  }, [baseId, highlightedIndex, open, kbNav])

  function handleClose() {
    setOpen(false)
    onClose?.()
  }

  function updateValues(next: T[]) {
    if (field.onValueChange) {
      field.onValueChange(next)
    } else {
      onChange(next)
    }
  }

  function renderMenuContent() {
    return (
      <>
        {field.searchable !== false ? (
          <>
            <Input
              role="combobox"
              aria-autocomplete="list"
              aria-expanded={true}
              aria-haspopup="listbox"
              aria-controls={`${baseId}-listbox`}
              aria-activedescendant={
                highlightedIndex >= 0
                  ? `${baseId}-item-${highlightedIndex}`
                  : undefined
              }
              placeholder={context.i18n.placeholders.searchField(
                field.label ?? ""
              )}
              className={cn(
                "h-8 rounded-none border-0 bg-transparent! px-2 text-sm shadow-none",
                "focus-visible:border-border focus-visible:ring-0 focus-visible:ring-offset-0"
              )}
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              onClick={(event) => event.stopPropagation()}
              onKeyDown={(event) => {
                if (event.key === "ArrowDown") {
                  event.preventDefault()
                  setKbNav(true)
                  if (allFilteredOptions.length > 0) {
                    setHighlightedIndex((previous) =>
                      previous < allFilteredOptions.length - 1
                        ? previous + 1
                        : 0
                    )
                  }
                } else if (event.key === "ArrowUp") {
                  event.preventDefault()
                  setKbNav(true)
                  if (allFilteredOptions.length > 0) {
                    setHighlightedIndex((previous) =>
                      previous > 0
                        ? previous - 1
                        : allFilteredOptions.length - 1
                    )
                  }
                } else if (event.key === "ArrowLeft") {
                  event.preventDefault()
                  setOpen(false)
                } else if (event.key === "Enter" && highlightedIndex >= 0) {
                  event.preventDefault()
                  const option = allFilteredOptions[highlightedIndex]
                  if (option) {
                    const isSelected = effectiveValues.includes(option.value)
                    const next = isSelected
                      ? effectiveValues.filter(
                          (value) => value !== option.value
                        )
                      : isMultiSelect
                        ? [...effectiveValues, option.value]
                        : [option.value]

                    if (
                      !isSelected &&
                      isMultiSelect &&
                      field.maxSelections &&
                      next.length > field.maxSelections
                    ) {
                      return
                    }

                    updateValues(next as T[])
                    if (!isMultiSelect) handleClose()
                  }
                }
                event.stopPropagation()
              }}
            />
            <DropdownMenuSeparator />
          </>
        ) : null}
        <div className="relative flex max-h-full">
          <div
            className="flex max-h-[min(var(--radix-dropdown-menu-content-available-height),24rem)] w-full scroll-pt-2 scroll-pb-2 flex-col overscroll-contain"
            role="listbox"
            id={`${baseId}-listbox`}
          >
            <ScrollArea className="size-full min-h-0 **:data-[slot=scroll-area-scrollbar]:m-0 **:data-[slot=scroll-area-viewport]:h-full **:data-[slot=scroll-area-viewport]:overscroll-contain">
              {allFilteredOptions.length === 0 ? (
                <div className="py-2 text-center text-sm text-muted-foreground">
                  {context.i18n.noResultsFound}
                </div>
              ) : null}

              {filteredSelectedOptions.length > 0 ? (
                <DropdownMenuGroup className="px-1">
                  {filteredSelectedOptions.map((option, index) => (
                    <DropdownMenuCheckboxItem
                      key={String(option.value)}
                      id={`${baseId}-item-${index}`}
                      role="option"
                      aria-selected={highlightedIndex === index}
                      data-highlighted={
                        kbNav && highlightedIndex === index ? true : undefined
                      }
                      checked={true}
                      className={cn(
                        "gap-1.5 data-highlighted:bg-accent",
                        option.className
                      )}
                      onSelect={(event) => {
                        if (isMultiSelect) event.preventDefault()
                      }}
                      onCheckedChange={() => {
                        const next = effectiveValues.filter(
                          (value) => value !== option.value
                        )
                        updateValues(next as T[])
                        if (!isMultiSelect) handleClose()
                      }}
                    >
                      {option.icon}
                      <span className="truncate">{option.label}</span>
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuGroup>
              ) : null}

              {filteredSelectedOptions.length > 0 &&
              filteredUnselectedOptions.length > 0 ? (
                <DropdownMenuSeparator className="mx-0" />
              ) : null}

              {filteredUnselectedOptions.length > 0 ? (
                <DropdownMenuGroup className="px-1">
                  {filteredUnselectedOptions.map((option, index) => {
                    const overallIndex = index + filteredSelectedOptions.length
                    return (
                      <DropdownMenuCheckboxItem
                        key={String(option.value)}
                        id={`${baseId}-item-${overallIndex}`}
                        role="option"
                        aria-selected={highlightedIndex === overallIndex}
                        data-highlighted={
                          kbNav && highlightedIndex === overallIndex
                            ? true
                            : undefined
                        }
                        checked={false}
                        className={cn(
                          "gap-1.5 data-highlighted:bg-accent",
                          option.className
                        )}
                        onSelect={(event) => {
                          if (isMultiSelect) event.preventDefault()
                        }}
                        onCheckedChange={() => {
                          const next = isMultiSelect
                            ? [...effectiveValues, option.value]
                            : [option.value]

                          if (
                            isMultiSelect &&
                            field.maxSelections &&
                            next.length > field.maxSelections
                          ) {
                            return
                          }

                          updateValues(next as T[])
                          if (!isMultiSelect) handleClose()
                        }}
                      >
                        {option.icon}
                        <span className="truncate">{option.label}</span>
                      </DropdownMenuCheckboxItem>
                    )
                  })}
                </DropdownMenuGroup>
              ) : null}
            </ScrollArea>
          </div>
        </div>
      </>
    )
  }

  if (inline) {
    return <div className="w-full">{renderMenuContent()}</div>
  }

  return (
    <DropdownMenu
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) {
          window.setTimeout(() => setSearchInput(""), 200)
        }
      }}
    >
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size={context.size}>
          <div className="flex items-center gap-1.5">
            {field.customValueRenderer ? (
              field.customValueRenderer(values, field.options ?? [])
            ) : (
              <>
                {selectedOptions.length > 0 ? (
                  <div className="flex items-center -space-x-1.5">
                    {selectedOptions.slice(0, 3).map((option) => (
                      <div key={String(option.value)}>{option.icon}</div>
                    ))}
                  </div>
                ) : null}
                {selectedOptions.length === 1
                  ? selectedOptions[0].label
                  : selectedOptions.length > 1
                    ? `${selectedOptions.length} ${context.i18n.selectedCount}`
                    : context.i18n.select}
              </>
            )}
          </div>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className={cn(
          "z-[200] w-[10.5rem] bg-popover px-0 before:hidden",
          field.className
        )}
      >
        {renderMenuContent()}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function FilterValueSelector<T = unknown>({
  field,
  values,
  onChange,
  operator,
  autoFocus,
}: {
  field: FilterFieldConfig<T>
  values: T[]
  onChange: (values: T[]) => void
  operator: string
  autoFocus?: boolean
}) {
  const context = useFilterContext()

  if (operator === "empty" || operator === "not_empty") return null

  if (field.customRenderer) {
    return (
      <ButtonGroupText
        className={cn(
          "bg-background text-start whitespace-nowrap outline-hidden hover:bg-accent aria-expanded:bg-accent dark:bg-input/30",
          filterControlHeight[context.size]
        )}
      >
        {field.customRenderer({ field, values, onChange, operator })}
      </ButtonGroupText>
    )
  }

  if (field.type === "text") {
    return (
      <FilterInput
        type="text"
        value={(values[0] as string) ?? ""}
        onChange={(event) => onChange([event.target.value] as T[])}
        placeholder={field.placeholder}
        pattern={field.pattern}
        field={field}
        className={cn("w-36", field.className)}
        autoFocus={autoFocus}
      />
    )
  }

  return (
    <SelectOptionsPopover field={field} values={values} onChange={onChange} />
  )
}

export interface FiltersContentProps<T = unknown> {
  filters: Filter<T>[]
  fields: FilterFieldsConfig<T>
  onChange: (filters: Filter<T>[]) => void
}

export function FiltersContent<T = unknown>({
  filters,
  fields,
  onChange,
}: FiltersContentProps<T>) {
  const context = useFilterContext()
  const fieldsMap = React.useMemo(() => getFieldsMap(fields), [fields])

  const updateFilter = React.useCallback(
    (filterId: string, updates: Partial<Filter<T>>) => {
      onChange(
        filters.map((filter) => {
          if (filter.id !== filterId) return filter
          const updatedFilter = { ...filter, ...updates }
          if (
            updates.operator === "empty" ||
            updates.operator === "not_empty"
          ) {
            updatedFilter.values = [] as T[]
          }
          return updatedFilter
        })
      )
    },
    [filters, onChange]
  )

  const removeFilter = React.useCallback(
    (filterId: string) => {
      onChange(filters.filter((filter) => filter.id !== filterId))
    },
    [filters, onChange]
  )

  return (
    <div
      className={cn(
        filtersContainerVariants({
          variant: context.variant,
          size: context.size,
        }),
        context.className
      )}
    >
      {filters.map((filter) => {
        const field = fieldsMap[filter.field]
        if (!field) return null

        return (
          <ButtonGroup key={filter.id}>
            <ButtonGroupText className={filterControlHeight[context.size]}>
              {field.icon}
              {field.label}
            </ButtonGroupText>
            <FilterOperatorDropdown<T>
              field={field}
              operator={filter.operator}
              values={filter.values}
              onChange={(operator) => updateFilter(filter.id, { operator })}
            />
            <FilterValueSelector<T>
              field={field}
              values={filter.values}
              onChange={(values) => updateFilter(filter.id, { values })}
              operator={filter.operator}
              autoFocus={false}
            />
            <FilterRemoveButton onClick={() => removeFilter(filter.id)} />
          </ButtonGroup>
        )
      })}
    </div>
  )
}

export interface FiltersProps<T = unknown> {
  filters: Filter<T>[]
  fields: FilterFieldsConfig<T>
  onChange: (filters: Filter<T>[]) => void
  className?: string
  variant?: "solid" | "default"
  size?: "sm" | "default" | "lg"
  radius?: "default" | "full"
  i18n?: Partial<
    Omit<
      FilterI18nConfig,
      "operators" | "placeholders" | "helpers" | "validation"
    >
  > & {
    operators?: Partial<FilterI18nConfig["operators"]>
    placeholders?: Partial<FilterI18nConfig["placeholders"]>
    helpers?: Partial<FilterI18nConfig["helpers"]>
    validation?: Partial<FilterI18nConfig["validation"]>
  }
  showSearchInput?: boolean
  trigger?: React.ReactNode
  allowMultiple?: boolean
  menuPopupClassName?: string
  collapseAddButton?: boolean
  enableShortcut?: boolean
  shortcutKey?: string
  shortcutLabel?: string
}

function FilterSubmenuContent<T = unknown>({
  field,
  currentValues,
  isMultiSelect,
  onToggle,
  i18n,
  isActive,
  onActive,
  onBack,
  onClose,
}: {
  field: FilterFieldConfig<T>
  currentValues: T[]
  isMultiSelect: boolean
  onToggle: (value: T, isSelected: boolean) => void
  i18n: FilterI18nConfig
  isActive?: boolean
  onActive?: () => void
  onBack?: () => void
  onClose?: () => void
}) {
  const [searchInput, setSearchInput] = React.useState("")
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1)
  const [kbNav, setKbNav] = React.useState(false)
  const inputRef = React.useRef<HTMLInputElement>(null)
  const baseId = React.useId()
  const filteredOptions = React.useMemo(() => {
    return (
      field.options?.filter((option) => {
        const isSelected = currentValues.includes(option.value)
        if (isSelected || !searchInput) return true
        return option.label.toLowerCase().includes(searchInput.toLowerCase())
      }) ?? []
    )
  }, [currentValues, field.options, searchInput])

  // Reset the highlight when the search changes; default to the first option when
  // this field becomes active with results — adjusted during render (the
  // React-sanctioned alternative to a setState-in-effect).
  const [prevSearchInput, setPrevSearchInput] = React.useState(searchInput)
  if (prevSearchInput !== searchInput) {
    setPrevSearchInput(searchInput)
    setHighlightedIndex(-1)
  }
  const activeKey = `${isActive}:${filteredOptions.length}`
  const [prevActiveKey, setPrevActiveKey] = React.useState(activeKey)
  if (prevActiveKey !== activeKey) {
    setPrevActiveKey(activeKey)
    if (isActive && filteredOptions.length > 0) setHighlightedIndex(0)
  }

  React.useEffect(() => {
    if (highlightedIndex >= 0 && isActive && kbNav) {
      document
        .getElementById(`${baseId}-item-${highlightedIndex}`)
        ?.scrollIntoView({ block: "nearest" })
    }
  }, [baseId, highlightedIndex, isActive, kbNav])

  function handleOptionToggle(option: FilterOption<T>) {
    onToggle(option.value as T, currentValues.includes(option.value))
    if (!isMultiSelect) {
      onBack?.()
    }
  }

  function handleKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      setKbNav(true)
      if (filteredOptions.length > 0) {
        setHighlightedIndex((previous) =>
          previous < filteredOptions.length - 1 ? previous + 1 : 0
        )
      }
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setKbNav(true)
      if (filteredOptions.length > 0) {
        setHighlightedIndex((previous) =>
          previous > 0 ? previous - 1 : filteredOptions.length - 1
        )
      }
    } else if (event.key === "ArrowLeft") {
      event.preventDefault()
      onBack?.()
    } else if (event.key === "Enter" && highlightedIndex >= 0) {
      event.preventDefault()
      const option = filteredOptions[highlightedIndex]
      if (option) handleOptionToggle(option)
    } else if (event.key === "Escape") {
      event.preventDefault()
      onClose?.()
    }
    event.stopPropagation()
  }

  return (
    <div className="flex flex-col" onFocus={onActive}>
      {field.searchable !== false ? (
        <>
          <Input
            ref={inputRef}
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={true}
            aria-haspopup="listbox"
            aria-controls={`${baseId}-listbox`}
            aria-activedescendant={
              highlightedIndex >= 0
                ? `${baseId}-item-${highlightedIndex}`
                : undefined
            }
            placeholder={i18n.placeholders.searchField(field.label ?? "")}
            className={cn(
              "h-8 rounded-none border-0 bg-transparent! px-2 text-sm shadow-none",
              "focus-visible:border-border focus-visible:ring-0 focus-visible:ring-offset-0"
            )}
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={handleKeyDown}
          />
          <DropdownMenuSeparator />
        </>
      ) : null}
      <div className="relative flex max-h-full">
        <div
          className="flex max-h-[min(var(--radix-dropdown-menu-content-available-height),24rem)] w-full scroll-pt-2 scroll-pb-2 flex-col overscroll-contain outline-hidden"
          role="listbox"
          id={`${baseId}-listbox`}
          tabIndex={field.searchable === false ? 0 : -1}
          onKeyDown={field.searchable === false ? handleKeyDown : undefined}
        >
          <ScrollArea className="size-full min-h-0 **:data-[slot=scroll-area-scrollbar]:m-0 **:data-[slot=scroll-area-viewport]:h-full **:data-[slot=scroll-area-viewport]:overscroll-contain">
            {filteredOptions.length === 0 ? (
              <div className="py-2 text-center text-sm text-muted-foreground">
                {i18n.noResultsFound}
              </div>
            ) : (
              <DropdownMenuGroup>
                {filteredOptions.map((option, index) => {
                  const isSelected = currentValues.includes(option.value)
                  const isHighlighted = highlightedIndex === index

                  return (
                    <DropdownMenuCheckboxItem
                      key={String(option.value)}
                      id={`${baseId}-item-${index}`}
                      role="option"
                      aria-selected={isHighlighted}
                      data-highlighted={
                        kbNav && isHighlighted ? true : undefined
                      }
                      checked={isSelected}
                      className={cn(
                        "gap-1.5 data-highlighted:bg-accent",
                        option.className
                      )}
                      onSelect={(event) => {
                        if (isMultiSelect) event.preventDefault()
                      }}
                      onCheckedChange={() => handleOptionToggle(option)}
                    >
                      {option.icon}
                      <span className="truncate">{option.label}</span>
                    </DropdownMenuCheckboxItem>
                  )
                })}
              </DropdownMenuGroup>
            )}
          </ScrollArea>
        </div>
      </div>
    </div>
  )
}

export function Filters<T = unknown>({
  filters,
  fields,
  onChange,
  className,
  variant = "default",
  size = "default",
  radius = "default",
  i18n,
  showSearchInput = true,
  trigger,
  allowMultiple = true,
  menuPopupClassName,
  enableShortcut = false,
  shortcutKey = "f",
  shortcutLabel = "F",
}: FiltersProps<T>) {
  const [addFilterOpen, setAddFilterOpen] = React.useState(false)
  const [menuSearchInput, setMenuSearchInput] = React.useState("")
  const [activeMenu, setActiveMenu] = React.useState("root")
  const [openSubMenu, setOpenSubMenu] = React.useState<string | null>(null)
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1)
  const [kbNav, setKbNav] = React.useState(false)
  const [lastAddedFilterId, setLastAddedFilterId] = React.useState<
    string | null
  >(null)
  const [sessionFilterIds, setSessionFilterIds] = React.useState<
    Record<string, string>
  >({})
  const rootInputRef = React.useRef<HTMLInputElement>(null)
  const rootId = React.useId()
  const mergedI18n: FilterI18nConfig = {
    ...DEFAULT_I18N,
    ...i18n,
    operators: { ...DEFAULT_I18N.operators, ...i18n?.operators },
    placeholders: { ...DEFAULT_I18N.placeholders, ...i18n?.placeholders },
    validation: { ...DEFAULT_I18N.validation, ...i18n?.validation },
    helpers: { ...DEFAULT_I18N.helpers, ...i18n?.helpers },
  }
  const fieldsMap = React.useMemo(() => getFieldsMap(fields), [fields])
  const selectableFields = React.useMemo(() => {
    return flattenFields(fields).filter((field) => {
      if (!field.key || field.type === "separator") return false
      if (allowMultiple) return true
      return !filters.some((filter) => filter.field === field.key)
    })
  }, [allowMultiple, fields, filters])
  const filteredFields = React.useMemo(() => {
    return selectableFields.filter(
      (field) =>
        !menuSearchInput ||
        field.label?.toLowerCase().includes(menuSearchInput.toLowerCase())
    )
  }, [menuSearchInput, selectableFields])

  React.useEffect(() => {
    if (!enableShortcut) return undefined

    function handleKeyDown(event: KeyboardEvent) {
      if (
        event.key.toLowerCase() === shortcutKey.toLowerCase() &&
        !addFilterOpen &&
        !(
          document.activeElement instanceof HTMLInputElement ||
          document.activeElement instanceof HTMLTextAreaElement
        )
      ) {
        event.preventDefault()
        setAddFilterOpen(true)
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [addFilterOpen, enableShortcut, shortcutKey])

  // Reset the highlight when the menu search changes (adjusted during render —
  // the React-sanctioned alternative to a setState-in-effect).
  const [prevMenuSearchInput, setPrevMenuSearchInput] =
    React.useState(menuSearchInput)
  if (prevMenuSearchInput !== menuSearchInput) {
    setPrevMenuSearchInput(menuSearchInput)
    setHighlightedIndex(-1)
  }

  React.useEffect(() => {
    if (highlightedIndex >= 0 && addFilterOpen && kbNav) {
      document
        .getElementById(`${rootId}-item-${highlightedIndex}`)
        ?.scrollIntoView({ block: "nearest" })
    }
  }, [addFilterOpen, highlightedIndex, rootId, kbNav])

  // Collapse any open submenu when the menu closes (adjusted during render).
  const [prevAddFilterOpen, setPrevAddFilterOpen] =
    React.useState(addFilterOpen)
  if (prevAddFilterOpen !== addFilterOpen) {
    setPrevAddFilterOpen(addFilterOpen)
    if (!addFilterOpen) setOpenSubMenu(null)
  }

  React.useEffect(() => {
    if (!lastAddedFilterId) return undefined
    const timer = window.setTimeout(() => setLastAddedFilterId(null), 1000)
    return () => window.clearTimeout(timer)
  }, [lastAddedFilterId])

  // Default to the first field when the menu opens with results (adjusted during
  // render).
  const fieldsActiveKey = `${addFilterOpen}:${filteredFields.length}`
  const [prevFieldsActiveKey, setPrevFieldsActiveKey] =
    React.useState(fieldsActiveKey)
  if (prevFieldsActiveKey !== fieldsActiveKey) {
    setPrevFieldsActiveKey(fieldsActiveKey)
    if (addFilterOpen && filteredFields.length > 0) setHighlightedIndex(0)
  }

  const updateFilter = React.useCallback(
    (filterId: string, updates: Partial<Filter<T>>) => {
      onChange(
        filters.map((filter) => {
          if (filter.id !== filterId) return filter
          const updatedFilter = { ...filter, ...updates }
          if (
            updates.operator === "empty" ||
            updates.operator === "not_empty"
          ) {
            updatedFilter.values = [] as T[]
          }
          return updatedFilter
        })
      )
    },
    [filters, onChange]
  )

  const removeFilter = React.useCallback(
    (filterId: string) => {
      onChange(filters.filter((filter) => filter.id !== filterId))
    },
    [filters, onChange]
  )

  const addFilter = React.useCallback(
    (fieldKey: string) => {
      const field = fieldsMap[fieldKey]
      if (!field?.key) return

      const defaultOperator =
        field.defaultOperator ??
        (field.type === "multiselect" ? "is_any_of" : "is")
      const defaultValues: unknown[] = field.type === "text" ? [""] : []
      const newFilter = createFilter<T>(
        fieldKey,
        defaultOperator,
        defaultValues as T[]
      )

      setLastAddedFilterId(newFilter.id)
      onChange([...filters, newFilter])
      setAddFilterOpen(false)
      setMenuSearchInput("")
    },
    [fieldsMap, filters, onChange]
  )

  function handleRootKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      setKbNav(true)
      if (filteredFields.length > 0) {
        setHighlightedIndex((previous) =>
          previous < filteredFields.length - 1 ? previous + 1 : 0
        )
      }
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setKbNav(true)
      if (filteredFields.length > 0) {
        setHighlightedIndex((previous) =>
          previous > 0 ? previous - 1 : filteredFields.length - 1
        )
      }
    } else if (
      (event.key === "ArrowRight" || event.key === "ArrowLeft") &&
      highlightedIndex >= 0
    ) {
      const field = filteredFields[highlightedIndex]
      const hasSubMenu =
        field &&
        (field.type === "select" || field.type === "multiselect") &&
        field.options?.length

      if (event.key === "ArrowRight" && hasSubMenu) {
        event.preventDefault()
        setOpenSubMenu(field.key ?? null)
        setActiveMenu(field.key ?? "root")
      } else if (event.key === "ArrowLeft") {
        event.preventDefault()
        if (openSubMenu) {
          setOpenSubMenu(null)
          setActiveMenu("root")
        }
      }
    } else if (event.key === "Enter" && highlightedIndex >= 0) {
      event.preventDefault()
      const field = filteredFields[highlightedIndex]
      if (field.key) {
        const hasSubMenu =
          (field.type === "select" || field.type === "multiselect") &&
          field.options?.length
        if (!hasSubMenu) {
          addFilter(field.key)
        } else {
          setOpenSubMenu((previous) =>
            previous === field.key ? null : (field.key ?? null)
          )
          setActiveMenu((previous) =>
            previous === field.key ? "root" : (field.key ?? "root")
          )
        }
      }
    } else if (event.key === "Escape") {
      setAddFilterOpen(false)
    }
    event.stopPropagation()
  }

  return (
    <FilterContext.Provider
      value={{
        variant,
        size,
        radius,
        i18n: mergedI18n,
        className,
        trigger,
        allowMultiple,
        showSearchInput,
      }}
    >
      <div
        className={cn(filtersContainerVariants({ variant, size }), className)}
      >
        {selectableFields.length > 0 ? (
          <DropdownMenu
            open={addFilterOpen}
            onOpenChange={(open) => {
              setAddFilterOpen(open)
              if (!open) {
                setMenuSearchInput("")
                setSessionFilterIds({})
              } else {
                setActiveMenu("root")
              }
            }}
          >
            <DropdownMenuTrigger asChild>
              {trigger ?? (
                <Button variant="outline" size={size}>
                  <PlusIcon />
                  {mergedI18n.addFilter}
                </Button>
              )}
            </DropdownMenuTrigger>
            <DropdownMenuContent
              className={cn(
                "z-[200] w-[10.5rem] bg-popover before:hidden",
                menuPopupClassName
              )}
              align="start"
            >
              {showSearchInput ? (
                <>
                  <div className="relative">
                    <Input
                      ref={rootInputRef}
                      role="combobox"
                      aria-controls={`${rootId}-listbox`}
                      aria-activedescendant={
                        highlightedIndex >= 0
                          ? `${rootId}-item-${highlightedIndex}`
                          : undefined
                      }
                      placeholder={mergedI18n.searchFields}
                      className={cn(
                        "h-8 rounded-none border-0 bg-transparent! px-2 text-sm shadow-none",
                        "focus-visible:border-border focus-visible:ring-0 focus-visible:ring-offset-0"
                      )}
                      value={menuSearchInput}
                      onChange={(event) =>
                        setMenuSearchInput(event.target.value)
                      }
                      onClick={(event) => event.stopPropagation()}
                      onKeyDown={handleRootKeyDown}
                    />
                    {enableShortcut && shortcutLabel ? (
                      <Kbd className="absolute top-1/2 right-2 -translate-y-1/2 border bg-background">
                        {shortcutLabel}
                      </Kbd>
                    ) : null}
                  </div>
                  <DropdownMenuSeparator />
                </>
              ) : null}

              <div className="relative flex max-h-full">
                <div
                  className="flex max-h-[min(var(--radix-dropdown-menu-content-available-height),24rem)] w-full scroll-pt-2 scroll-pb-2 flex-col overscroll-contain"
                  role="listbox"
                  id={`${rootId}-listbox`}
                >
                  <ScrollArea className="**:data-[slot=scroll-area-scrollbar]:m-0">
                    {filteredFields.length === 0 ? (
                      <div className="py-2 text-center text-sm text-muted-foreground">
                        {mergedI18n.noFieldsFound}
                      </div>
                    ) : (
                      filteredFields.map((field, index) => {
                        const isHighlighted = highlightedIndex === index
                        const itemId = `${rootId}-item-${index}`
                        const hasSubMenu =
                          (field.type === "select" ||
                            field.type === "multiselect") &&
                          field.options?.length

                        if (hasSubMenu) {
                          const isMultiSelect = field.type === "multiselect"
                          const fieldKey = field.key as string
                          const sessionFilterId = sessionFilterIds[fieldKey]
                          const sessionFilter = sessionFilterId
                            ? filters.find(
                                (filter) => filter.id === sessionFilterId
                              )
                            : null
                          const currentValues = sessionFilter?.values ?? []

                          return (
                            <DropdownMenuSub
                              key={fieldKey}
                              open={openSubMenu === fieldKey}
                              onOpenChange={(open) => {
                                if (open) {
                                  setOpenSubMenu((previous) =>
                                    previous === fieldKey ? previous : fieldKey
                                  )
                                } else if (openSubMenu === fieldKey) {
                                  setOpenSubMenu(null)
                                  setActiveMenu("root")
                                }
                              }}
                            >
                              <DropdownMenuSubTrigger
                                id={itemId}
                                role="option"
                                aria-selected={isHighlighted}
                                data-highlighted={
                                  kbNav && isHighlighted ? true : undefined
                                }
                                className="gap-1.5 data-highlighted:bg-accent data-highlighted:text-accent-foreground data-[state=open]:bg-accent data-[state=open]:text-accent-foreground"
                              >
                                {field.icon}
                                <span>{field.label}</span>
                              </DropdownMenuSubTrigger>
                              <DropdownMenuSubContent
                                collisionPadding={12}
                                sideOffset={6}
                                className="z-[200] w-[min(10rem,calc(100vw-1.5rem))] bg-popover before:hidden"
                              >
                                <FilterSubmenuContent
                                  field={field}
                                  currentValues={currentValues}
                                  isMultiSelect={isMultiSelect}
                                  i18n={mergedI18n}
                                  isActive={activeMenu === fieldKey}
                                  onActive={() => {
                                    if (field.searchable !== false) {
                                      setActiveMenu(fieldKey)
                                    }
                                  }}
                                  onBack={() => {
                                    setOpenSubMenu(null)
                                    setActiveMenu("root")
                                  }}
                                  onClose={() => setAddFilterOpen(false)}
                                  onToggle={(value, isSelected) => {
                                    if (isMultiSelect) {
                                      const nextValues = isSelected
                                        ? (currentValues.filter(
                                            (item) => item !== value
                                          ) as T[])
                                        : ([...currentValues, value] as T[])

                                      if (sessionFilter) {
                                        if (nextValues.length === 0) {
                                          onChange(
                                            filters.filter(
                                              (filter) =>
                                                filter.id !== sessionFilter.id
                                            )
                                          )
                                          setSessionFilterIds((previous) => ({
                                            ...previous,
                                            [fieldKey]: "",
                                          }))
                                        } else {
                                          onChange(
                                            filters.map((filter) =>
                                              filter.id === sessionFilter.id
                                                ? {
                                                    ...filter,
                                                    values: nextValues,
                                                  }
                                                : filter
                                            )
                                          )
                                        }
                                      } else {
                                        const newFilter = createFilter<T>(
                                          fieldKey,
                                          field.defaultOperator ?? "is_any_of",
                                          nextValues
                                        )
                                        onChange([...filters, newFilter])
                                        setSessionFilterIds((previous) => ({
                                          ...previous,
                                          [fieldKey]: newFilter.id,
                                        }))
                                      }
                                    } else {
                                      const newFilter = createFilter<T>(
                                        fieldKey,
                                        field.defaultOperator ?? "is",
                                        [value] as T[]
                                      )
                                      setLastAddedFilterId(newFilter.id)
                                      onChange([...filters, newFilter])
                                      setAddFilterOpen(false)
                                    }
                                  }}
                                />
                              </DropdownMenuSubContent>
                            </DropdownMenuSub>
                          )
                        }

                        return (
                          <DropdownMenuItem
                            key={field.key}
                            id={itemId}
                            role="option"
                            aria-selected={isHighlighted}
                            data-highlighted={
                              kbNav && isHighlighted ? true : undefined
                            }
                            onClick={() => field.key && addFilter(field.key)}
                            className="gap-1.5 data-highlighted:bg-accent data-highlighted:text-accent-foreground"
                          >
                            {field.icon}
                            <span>{field.label}</span>
                          </DropdownMenuItem>
                        )
                      })
                    )}
                  </ScrollArea>
                </div>
              </div>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null}

        {filters.map((filter) => {
          const field = fieldsMap[filter.field]
          if (!field) return null

          return (
            <ButtonGroup key={filter.id}>
              <ButtonGroupText
                className={cn(
                  "bg-background dark:bg-input/30",
                  filterControlHeight[size]
                )}
              >
                {field.icon}
                {field.label}
              </ButtonGroupText>
              <FilterOperatorDropdown<T>
                field={field}
                operator={filter.operator}
                values={filter.values}
                onChange={(operator) => updateFilter(filter.id, { operator })}
              />
              <FilterValueSelector<T>
                field={field}
                values={filter.values}
                operator={filter.operator}
                onChange={(values) => updateFilter(filter.id, { values })}
                autoFocus={filter.id === lastAddedFilterId}
              />
              <FilterRemoveButton onClick={() => removeFilter(filter.id)} />
            </ButtonGroup>
          )
        })}
      </div>
    </FilterContext.Provider>
  )
}

export function createFilter<T = unknown>(
  field: string,
  operator?: string,
  values: T[] = []
): Filter<T> {
  return {
    id: `${Date.now()}-${Math.random().toString(36).substring(2, 11)}`,
    field,
    operator: operator ?? "is",
    values,
  }
}

export function createFilterGroup<T = unknown>(
  id: string,
  label: string,
  fields: FilterFieldConfig<T>[],
  initialFilters: Filter<T>[] = []
): FilterGroup<T> {
  return {
    id,
    label,
    filters: initialFilters,
    fields,
  }
}
