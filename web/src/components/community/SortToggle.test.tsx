import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SortToggle } from './SortToggle'

describe('SortToggle', () => {
  it('点击切换触发 onChange', () => {
    const onChange = vi.fn()
    render(<SortToggle value="new" onChange={onChange} />)
    fireEvent.click(screen.getByRole('button', { name: /最热/ }))
    expect(onChange).toHaveBeenCalledWith('hot')
  })
})
