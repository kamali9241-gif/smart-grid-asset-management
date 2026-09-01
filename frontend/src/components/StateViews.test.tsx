import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StateViews } from '../test/StateViewsWrapper'

describe('StateViews', () => {
  it('renders loading, empty and error states with accessible roles', () => {
    render(<StateViews />)
    expect(screen.getAllByRole('status')).toHaveLength(2)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.getByText('Nothing here.')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('Something broke.')
  })
})
